package test

import (
	"os"
	"runtime"
	"testing"

	statsig_go "github.com/statsig-io/statsig-go-core"
)

// Covers the EnforceOverrides / EnforceTargeting persistent-assignment
// options. Fixture (enforce_sticky_dcs.json): experiment `enforce_exp` with a
// console override rule matching userID `override-user`, a targeting gate
// passing only users with custom `targeted=yes`, and layer `enforce_layer`
// delegating to the experiment.

type noopPersistentStorage struct{}

func (m *noopPersistentStorage) GetFunctions() statsig_go.PersistentStorageFunctions {
	return statsig_go.PersistentStorageFunctions{
		Load:   func(key string) *statsig_go.UserPersistedValues { return nil },
		Save:   func(key string, configName string, data statsig_go.StickyValues) {},
		Delete: func(key string, configName string) {},
	}
}

func setupEnforceTest(t *testing.T) *statsig_go.Statsig {
	resData, err := os.ReadFile("data/enforce_sticky_dcs.json")
	if err != nil {
		t.Fatalf("error reading fixture: %v", err)
	}

	scrapi := NewMockScrapi()
	t.Cleanup(scrapi.Close)
	scrapi.Stub("GET", "/v2/download_config_specs/secret-123.json", StubResponse{
		Status: 200,
		Body:   resData,
	})
	scrapi.Stub("POST", "/v1/log_event", StubResponse{
		Status: 200,
		Body:   []byte(`{"success": true}`),
	})

	storage := statsig_go.NewPersistentStorage((&noopPersistentStorage{}).GetFunctions())
	// PersistentStorage has a finalizer that releases the underlying native
	// instance; keep it reachable until the test finishes so GC cannot release
	// it while the Statsig instance is still using it.
	t.Cleanup(func() { runtime.KeepAlive(storage) })

	opts, err := statsig_go.NewOptionsBuilder().
		WithSpecsUrl(scrapi.URL() + "/v2/download_config_specs").
		WithLogEventUrl(scrapi.URL() + "/v1/log_event").
		// UserPersistedValues are only honored when a persistent storage
		// adapter is configured.
		WithPersistentStorage(storage).
		Build()
	if err != nil {
		t.Fatalf("error creating StatsigOptions: %v", err)
	}

	statsig, err := statsig_go.NewStatsigWithOptions("secret-123", opts)
	if err != nil {
		t.Fatalf("error creating Statsig: %v", err)
	}

	statsig.Initialize()
	return statsig
}

func makeEnforceUser(t *testing.T, userID string, targeted bool) *statsig_go.StatsigUser {
	targetedValue := "no"
	if targeted {
		targetedValue = "yes"
	}

	user, err := statsig_go.NewUserBuilderWithUserID(userID).
		WithCustom(map[string]any{"targeted": targetedValue}).
		Build()
	if err != nil {
		t.Fatalf("error creating StatsigUser: %v", err)
	}
	return user
}

func stickyValuesFor(configName string, configDelegate *string) *statsig_go.UserPersistedValues {
	groupName := "Sticky Group"
	values := statsig_go.UserPersistedValues{
		configName: {
			Value:                         true,
			JSONValue:                     map[string]string{"value": "sticky_value"},
			RuleID:                        "sticky_rule_id",
			GroupName:                     &groupName,
			SecondaryExposures:            []statsig_go.SecondaryExposure{},
			UndelegatedSecondaryExposures: []statsig_go.SecondaryExposure{},
			ConfigDelegate:                configDelegate,
			Time:                          1700000000000,
		},
	}
	return &values
}

func TestStickyValueWinsWithoutEnforceOverrides(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	experiment := statsig.GetExperimentWithOptions(
		makeEnforceUser(t, "override-user", true),
		"enforce_exp",
		&statsig_go.ExperimentEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_exp", nil),
		},
	)

	if v := experiment.GetString("value", "err"); v != "sticky_value" {
		t.Errorf("Expected 'sticky_value', got '%s'", v)
	}
	if experiment.RuleID != "sticky_rule_id" {
		t.Errorf("Expected rule 'sticky_rule_id', got '%s'", experiment.RuleID)
	}
}

func TestEnforceOverridesLetsOverrideWinOverSticky(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	experiment := statsig.GetExperimentWithOptions(
		makeEnforceUser(t, "override-user", true),
		"enforce_exp",
		&statsig_go.ExperimentEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_exp", nil),
			EnforceOverrides:    true,
		},
	)

	if v := experiment.GetString("value", "err"); v != "override_value" {
		t.Errorf("Expected 'override_value', got '%s'", v)
	}
	if experiment.RuleID != "override_rule:userID:id_override" {
		t.Errorf("Expected the override rule, got '%s'", experiment.RuleID)
	}
}

func TestEnforceOverridesKeepsStickyWhenNoOverrideMatches(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	experiment := statsig.GetExperimentWithOptions(
		makeEnforceUser(t, "plain-user", true),
		"enforce_exp",
		&statsig_go.ExperimentEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_exp", nil),
			EnforceOverrides:    true,
		},
	)

	if v := experiment.GetString("value", "err"); v != "sticky_value" {
		t.Errorf("Expected 'sticky_value', got '%s'", v)
	}
}

func TestEnforceTargetingKeepsStickyWhenStillTargeted(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	experiment := statsig.GetExperimentWithOptions(
		makeEnforceUser(t, "plain-user", true),
		"enforce_exp",
		&statsig_go.ExperimentEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_exp", nil),
			EnforceTargeting:    true,
		},
	)

	if v := experiment.GetString("value", "err"); v != "sticky_value" {
		t.Errorf("Expected 'sticky_value', got '%s'", v)
	}
}

func TestEnforceTargetingDropsStickyWhenNoLongerTargeted(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	experiment := statsig.GetExperimentWithOptions(
		makeEnforceUser(t, "plain-user", false),
		"enforce_exp",
		&statsig_go.ExperimentEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_exp", nil),
			EnforceTargeting:    true,
		},
	)

	if v := experiment.GetString("value", "err"); v == "sticky_value" {
		t.Errorf("Expected the sticky value to be dropped, got '%s'", v)
	}
	if experiment.RuleID != "targetingGate" {
		t.Errorf("Expected rule 'targetingGate', got '%s'", experiment.RuleID)
	}
}

func TestLayerStickyValueWinsWithoutEnforceOverrides(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	delegate := "enforce_exp"
	layer := statsig.GetLayerWithOptions(
		makeEnforceUser(t, "override-user", true),
		"enforce_layer",
		&statsig_go.LayerEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_layer", &delegate),
		},
	)

	if v := layer.GetString("value", "err"); v != "sticky_value" {
		t.Errorf("Expected 'sticky_value', got '%s'", v)
	}
}

func TestLayerEnforceOverridesLetsOverrideWinOverSticky(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	delegate := "enforce_exp"
	layer := statsig.GetLayerWithOptions(
		makeEnforceUser(t, "override-user", true),
		"enforce_layer",
		&statsig_go.LayerEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_layer", &delegate),
			EnforceOverrides:    true,
		},
	)

	if v := layer.GetString("value", "err"); v != "override_value" {
		t.Errorf("Expected 'override_value', got '%s'", v)
	}
}

func TestLayerEnforceTargetingDropsStickyWhenNoLongerTargeted(t *testing.T) {
	statsig := setupEnforceTest(t)
	defer statsig.Shutdown()

	delegate := "enforce_exp"
	layer := statsig.GetLayerWithOptions(
		makeEnforceUser(t, "plain-user", false),
		"enforce_layer",
		&statsig_go.LayerEvaluationOptions{
			UserPersistedValues: stickyValuesFor("enforce_layer", &delegate),
			EnforceTargeting:    true,
		},
	)

	if v := layer.GetString("value", "err"); v == "sticky_value" {
		t.Errorf("Expected the sticky value to be dropped, got '%s'", v)
	}
}
