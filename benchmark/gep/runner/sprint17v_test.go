package runner

import (
	"testing"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

func TestRunConfigDefaults(t *testing.T) {
	cfg := RunConfig{
		Model:    "test-model",
		Provider: types.ProviderOllama,
	}
	// Verify defaults are applied
	if len(cfg.Conditions) != 0 {
		t.Error("expected empty conditions before Run() call")
	}
}

func TestDefaultConditions(t *testing.T) {
	conds := defaultConditions()
	if len(conds) != 2 {
		t.Errorf("expected 2 default conditions, got %d", len(conds))
	}
	if conds[0] != types.ConditionDirect {
		t.Errorf("expected first condition to be direct, got %s", conds[0])
	}
	if conds[1] != types.ConditionGumiStabilized {
		t.Errorf("expected second condition to be gumi-stabilized, got %s", conds[1])
	}
}

func TestComputeSummaryWithBothConditions(t *testing.T) {
	results := []types.GEPResult{
		{TestID: "t1", Condition: types.ConditionDirect, Passed: true, Subscores: map[string]float64{"a": 1.0}, LatencyMs: 100},
		{TestID: "t2", Condition: types.ConditionDirect, Passed: false, Subscores: map[string]float64{"b": 0.0}, LatencyMs: 150},
		{TestID: "t3", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"a": 1.0}, LatencyMs: 120},
		{TestID: "t4", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"b": 1.0}, LatencyMs: 130},
	}
	summary := computeSummary(results, []types.GEPCondition{types.ConditionDirect, types.ConditionGumiStabilized})

	if summary.DirectScore != 0.5 {
		t.Errorf("expected direct score 0.5, got %f", summary.DirectScore)
	}
	if summary.GumiScore != 1.0 {
		t.Errorf("expected gumi score 1.0, got %f", summary.GumiScore)
	}
	if summary.ScoreDelta != 0.5 {
		t.Errorf("expected score delta 0.5, got %f", summary.ScoreDelta)
	}
	if summary.DirectPassRate != 0.5 {
		t.Errorf("expected direct pass rate 0.5, got %f", summary.DirectPassRate)
	}
	if summary.GumiPassRate != 1.0 {
		t.Errorf("expected gumi pass rate 1.0, got %f", summary.GumiPassRate)
	}
	if summary.PassRateDelta != 0.5 {
		t.Errorf("expected pass rate delta 0.5, got %f", summary.PassRateDelta)
	}
}

func TestComputeSummaryOnlyDirect(t *testing.T) {
	results := []types.GEPResult{
		{TestID: "t1", Condition: types.ConditionDirect, Passed: true, Subscores: map[string]float64{"a": 1.0}},
		{TestID: "t2", Condition: types.ConditionDirect, Passed: false, Subscores: map[string]float64{"b": 0.0}},
	}
	summary := computeSummary(results, []types.GEPCondition{types.ConditionDirect})

	if summary.DirectScore != 0.5 {
		t.Errorf("expected direct score 0.5, got %f", summary.DirectScore)
	}
	if summary.GumiScore != 0 {
		t.Errorf("expected gumi score 0 when no gumi data, got %f", summary.GumiScore)
	}
}

func TestComputeSummaryOnlyGumi(t *testing.T) {
	results := []types.GEPResult{
		{TestID: "t1", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"a": 1.0}},
		{TestID: "t2", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"b": 1.0}},
	}
	summary := computeSummary(results, []types.GEPCondition{types.ConditionGumiStabilized})

	if summary.GumiScore != 1.0 {
		t.Errorf("expected gumi score 1.0, got %f", summary.GumiScore)
	}
	if summary.DirectScore != 0 {
		t.Errorf("expected direct score 0 when no direct data, got %f", summary.DirectScore)
	}
}

func TestAggregateCapabilitiesWithConditions(t *testing.T) {
	results := []types.GEPResult{
		{TestID: "t1", SuiteID: "instruction_following", Condition: types.ConditionDirect, Passed: true, Subscores: map[string]float64{"a": 1.0}},
		{TestID: "t2", SuiteID: "instruction_following", Condition: types.ConditionDirect, Passed: false, Subscores: map[string]float64{"b": 0.0}},
		{TestID: "t3", SuiteID: "instruction_following", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"a": 1.0}},
		{TestID: "t4", SuiteID: "instruction_following", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"b": 1.0}},
		{TestID: "t5", SuiteID: "structured_output", Condition: types.ConditionDirect, Passed: true, Subscores: map[string]float64{"c": 1.0}},
		{TestID: "t6", SuiteID: "structured_output", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"c": 1.0}},
	}
	caps := aggregateCapabilities(results)

	// Check instruction_following
	if caps["instruction_following"].Direct.N != 2 {
		t.Errorf("expected 2 direct results, got %d", caps["instruction_following"].Direct.N)
	}
	if caps["instruction_following"].Gumi.N != 2 {
		t.Errorf("expected 2 gumi results, got %d", caps["instruction_following"].Gumi.N)
	}
	if caps["instruction_following"].Delta != 0.5 {
		t.Errorf("expected delta 0.5, got %f", caps["instruction_following"].Delta)
	}
	if caps["instruction_following"].PassRate != 0.5 {
		t.Errorf("expected pass rate delta 0.5, got %f", caps["instruction_following"].PassRate)
	}

	// Check structured_output
	if caps["structured_output"].Direct.N != 1 {
		t.Errorf("expected 1 direct result, got %d", caps["structured_output"].Direct.N)
	}
	if caps["structured_output"].Gumi.N != 1 {
		t.Errorf("expected 1 gumi result, got %d", caps["structured_output"].Gumi.N)
	}
	if caps["structured_output"].Delta != 0 {
		t.Errorf("expected delta 0 for identical scores, got %f", caps["structured_output"].Delta)
	}
}

func TestGEPConditionConstants(t *testing.T) {
	if types.ConditionDirect != "direct" {
		t.Errorf("expected ConditionDirect='direct', got %s", types.ConditionDirect)
	}
	if types.ConditionGumiStabilized != "gumi-stabilized" {
		t.Errorf("expected ConditionGumiStabilized='gumi-stabilized', got %s", types.ConditionGumiStabilized)
	}
}

func TestGEPScopeConstants(t *testing.T) {
	if types.ScopeModel != "model" {
		t.Errorf("expected ScopeModel='model', got %s", types.ScopeModel)
	}
	if types.ScopeRuntime != "runtime" {
		t.Errorf("expected ScopeRuntime='runtime', got %s", types.ScopeRuntime)
	}
}

func TestProtocolVersion(t *testing.T) {
	if types.ProtocolVersion != "2.0.0" {
		t.Errorf("expected ProtocolVersion='2.0.0', got %s", types.ProtocolVersion)
	}
}
