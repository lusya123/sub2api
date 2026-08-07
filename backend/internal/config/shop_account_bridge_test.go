package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestValidateShopAccountBridgeConfig_DefaultDisabled(t *testing.T) {
	require.NoError(t, validateShopAccountBridgeConfig(ShopAccountBridgeConfig{}))
}

func TestShopAccountBridgeConfig_LoadsFromEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("SHOP_ACCOUNT_BRIDGE_ENABLED", "true")
	t.Setenv("SHOP_ACCOUNT_BRIDGE_SHARED_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("SHOP_ACCOUNT_BRIDGE_CLOCK_SKEW_SECONDS", "45")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	setDefaults()

	var cfg Config
	require.NoError(t, viper.Unmarshal(&cfg))
	require.True(t, cfg.ShopAccountBridge.Enabled)
	require.Equal(t, "0123456789abcdef0123456789abcdef", cfg.ShopAccountBridge.SharedSecret)
	require.Equal(t, 45, cfg.ShopAccountBridge.ClockSkewSeconds)
}

func TestValidateShopAccountBridgeConfig_EnabledFailsClosed(t *testing.T) {
	valid := ShopAccountBridgeConfig{
		Enabled:          true,
		SharedSecret:     "0123456789abcdef0123456789abcdef",
		ClockSkewSeconds: 60,
	}
	require.NoError(t, validateShopAccountBridgeConfig(valid))

	tests := []struct {
		name   string
		mutate func(*ShopAccountBridgeConfig)
		want   string
	}{
		{name: "empty secret", mutate: func(c *ShopAccountBridgeConfig) { c.SharedSecret = "" }, want: "at least 32 bytes"},
		{name: "short secret", mutate: func(c *ShopAccountBridgeConfig) { c.SharedSecret = "short" }, want: "at least 32 bytes"},
		{name: "zero skew", mutate: func(c *ShopAccountBridgeConfig) { c.ClockSkewSeconds = 0 }, want: "between 1 and 300"},
		{name: "large skew", mutate: func(c *ShopAccountBridgeConfig) { c.ClockSkewSeconds = 301 }, want: "between 1 and 300"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			require.ErrorContains(t, validateShopAccountBridgeConfig(cfg), tc.want)
		})
	}
}
