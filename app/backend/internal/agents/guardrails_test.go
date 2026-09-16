package agents

import "testing"

func TestPositionGuardrailsForbidPositionAssumptionsWithoutInput(t *testing.T) {
	s := positionGuardrails(&AgentState{XPlayerTakes: "alice still holding CRWV"})
	if !contains(s, "alice still holding CRWV") {
		t.Fatalf("missing takes: %s", s)
	}
	for _, required := range []string{"No verified position", "conditional observation", "unknown", "well-established knowledge"} {
		if !contains(s, required) {
			t.Fatalf("missing %q guardrail: %s", required, s)
		}
	}
}

func TestSourceDisciplineRequiresUnknownsAndConditionalObservations(t *testing.T) {
	instruction := sourceDisciplineInstruction()
	for _, required := range []string{"well-established knowledge", "say unknown", "no verified position", "conditional observations only"} {
		if !contains(instruction, required) {
			t.Fatalf("missing %q source discipline: %s", required, instruction)
		}
	}
	for _, blocked := range []string{"HOLD", "sell 50%", "stop loss", "buy call option", "maintain the existing position", "维持现有仓位"} {
		if ConditionalObservationSafe(blocked) {
			t.Fatalf("unsafe no-position output passed publication guard: %q", blocked)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
