package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestValidateShopCredentialEventsConfig_DefaultDisabled(t *testing.T) {
	require.NoError(t, validateShopCredentialEventsConfig(ShopCredentialEventsConfig{}))
}

func TestShopCredentialEventsConfig_LoadsFromEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("SHOP_CREDENTIAL_EVENTS_ENABLED", "true")
	t.Setenv("SHOP_CREDENTIAL_EVENTS_BASE_URL", "https://shop.example.com")
	t.Setenv("SHOP_CREDENTIAL_EVENTS_SHARED_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("SHOP_CREDENTIAL_EVENTS_TIMEOUT_SECONDS", "12")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	setDefaults()

	var cfg Config
	require.NoError(t, viper.Unmarshal(&cfg))
	require.True(t, cfg.ShopCredentialEvents.Enabled)
	require.Equal(t, "https://shop.example.com", cfg.ShopCredentialEvents.BaseURL)
	require.Equal(t, "0123456789abcdef0123456789abcdef", cfg.ShopCredentialEvents.SharedSecret)
	require.Equal(t, 12, cfg.ShopCredentialEvents.TimeoutSeconds)
}

func TestValidateShopCredentialEventsConfig_EnabledRequiresSecureIndependentConfig(t *testing.T) {
	valid := ShopCredentialEventsConfig{
		Enabled:        true,
		BaseURL:        "https://shop.example.com",
		SharedSecret:   "0123456789abcdef0123456789abcdef",
		TimeoutSeconds: 10,
	}
	require.NoError(t, validateShopCredentialEventsConfig(valid))

	tests := []struct {
		name   string
		mutate func(*ShopCredentialEventsConfig)
		want   string
	}{
		{name: "http", mutate: func(c *ShopCredentialEventsConfig) { c.BaseURL = "http://shop.example.com" }, want: "HTTPS"},
		{name: "path", mutate: func(c *ShopCredentialEventsConfig) { c.BaseURL = "https://shop.example.com/prefix" }, want: "must not contain"},
		{name: "empty secret", mutate: func(c *ShopCredentialEventsConfig) { c.SharedSecret = "" }, want: "at least 32 bytes"},
		{name: "short secret", mutate: func(c *ShopCredentialEventsConfig) { c.SharedSecret = "short" }, want: "at least 32 bytes"},
		{name: "zero timeout", mutate: func(c *ShopCredentialEventsConfig) { c.TimeoutSeconds = 0 }, want: "between 1 and 60"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			require.ErrorContains(t, validateShopCredentialEventsConfig(cfg), tc.want)
		})
	}
}
