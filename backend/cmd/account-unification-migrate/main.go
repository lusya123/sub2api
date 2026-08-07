package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accountunification"
	_ "github.com/lib/pq"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "account-unification-migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "plan":
		return runPlan(args[1:])
	case "apply":
		return runApply(args[1:])
	default:
		return usageError()
	}
}

func runPlan(args []string) error {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	out := flags.String("out", "", "0600 JSON plan output path")
	timeout := flags.Duration("timeout", 2*time.Minute, "database operation timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*out) == "" {
		return errors.New("--out is required")
	}
	target, err := migrationTarget()
	if err != nil {
		return err
	}
	mainDB, shopDB, closeDBs, err := openDatabases()
	if err != nil {
		return err
	}
	defer closeDBs()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := pingBoth(ctx, mainDB, shopDB); err != nil {
		return err
	}
	plan, err := accountunification.BuildPlan(ctx, mainDB, shopDB, time.Now())
	if err != nil {
		return err
	}
	plan.Target = target
	digest, err := accountunification.WritePlan(*out, plan)
	if err != nil {
		return err
	}
	printJSON(map[string]any{
		"mode":        "plan",
		"target":      target,
		"plan_path":   *out,
		"plan_sha256": digest,
		"counts":      plan.Counts,
	})
	return nil
}

func runApply(args []string) error {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	planPath := flags.String("plan", "", "plan file produced by the plan command")
	planDigest := flags.String("plan-sha256", "", "exact SHA-256 printed by the plan command")
	confirm := flags.String("confirm", "", "exact mutation confirmation")
	maxUsers := flags.Int("max-users", 1, "maximum matched accounts to apply")
	applyAll := flags.Bool("all", false, "apply all eligible plan items")
	allowProduction := flags.Bool("allow-production", false, "allow a production-labeled plan after exact confirmation")
	resultPath := flags.String("result", "", "optional 0600 JSON result path")
	timeout := flags.Duration("timeout", 5*time.Minute, "database operation timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*planPath) == "" || strings.TrimSpace(*planDigest) == "" {
		return errors.New("--plan and --plan-sha256 are required")
	}
	plan, actualDigest, err := accountunification.ReadPlan(*planPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(*planDigest), actualDigest) {
		return errors.New("plan SHA-256 does not match")
	}
	target, err := migrationTarget()
	if err != nil {
		return err
	}
	if plan.Target == "" || !strings.EqualFold(plan.Target, target) {
		return fmt.Errorf("plan target %q does not match ACCOUNT_UNIFICATION_TARGET %q", plan.Target, target)
	}
	expectedConfirmation := "APPLY_MATCHED_ACCOUNTS_TO_" + strings.ToUpper(target)
	if *confirm != expectedConfirmation {
		return fmt.Errorf("--confirm must equal %q", expectedConfirmation)
	}
	if target == "production" && !*allowProduction {
		return errors.New("production apply requires --allow-production in addition to the exact confirmation")
	}
	mainDB, shopDB, closeDBs, err := openDatabases()
	if err != nil {
		return err
	}
	defer closeDBs()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := pingBoth(ctx, mainDB, shopDB); err != nil {
		return err
	}
	results, err := accountunification.Apply(ctx, mainDB, shopDB, plan, *maxUsers, *applyAll)
	if writeErr := accountunification.WriteResults(*resultPath, results); writeErr != nil && err == nil {
		err = writeErr
	}
	printJSON(map[string]any{
		"mode":          "apply",
		"target":        target,
		"applied_count": len(results),
		"result_path":   *resultPath,
	})
	return err
}

func migrationTarget() (string, error) {
	target := strings.ToLower(strings.TrimSpace(os.Getenv("ACCOUNT_UNIFICATION_TARGET")))
	if target == "" {
		return "", errors.New("ACCOUNT_UNIFICATION_TARGET must be set to an explicit environment label such as staging")
	}
	for _, r := range target {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return "", errors.New("ACCOUNT_UNIFICATION_TARGET may contain only lowercase letters, digits, '-' and '_'")
		}
	}
	return target, nil
}

func openDatabases() (*sql.DB, *sql.DB, func(), error) {
	mainDSN := strings.TrimSpace(os.Getenv("ACCOUNT_UNIFICATION_MAIN_DSN"))
	shopDSN := strings.TrimSpace(os.Getenv("ACCOUNT_UNIFICATION_SHOP_DSN"))
	if mainDSN == "" || shopDSN == "" {
		return nil, nil, func() {}, errors.New("ACCOUNT_UNIFICATION_MAIN_DSN and ACCOUNT_UNIFICATION_SHOP_DSN must be set")
	}
	mainDB, err := sql.Open("postgres", mainDSN)
	if err != nil {
		return nil, nil, func() {}, err
	}
	shopDB, err := sql.Open("postgres", shopDSN)
	if err != nil {
		mainDB.Close()
		return nil, nil, func() {}, err
	}
	mainDB.SetMaxOpenConns(2)
	shopDB.SetMaxOpenConns(2)
	return mainDB, shopDB, func() {
		mainDB.Close()
		shopDB.Close()
	}, nil
}

func pingBoth(ctx context.Context, mainDB, shopDB *sql.DB) error {
	if err := mainDB.PingContext(ctx); err != nil {
		return fmt.Errorf("connect Main database: %w", err)
	}
	if err := shopDB.PingContext(ctx); err != nil {
		return fmt.Errorf("connect Shop database: %w", err)
	}
	return nil
}

func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func usageError() error {
	return errors.New("usage: account-unification-migrate <plan|apply> [options]")
}
