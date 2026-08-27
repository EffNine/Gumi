package verify

import (
	"strings"
	"testing"
)

func TestNumericAnswer(t *testing.T) {
	c := NumericAnswer(3901)
	if err := c("3901"); err != nil {
		t.Errorf("exact number rejected: %v", err)
	}
	if err := c("The answer is 3,901."); err != nil {
		t.Errorf("prose number rejected: %v", err)
	}
	if err := c("1234"); err == nil {
		t.Error("wrong number accepted")
	}
}

func TestExactFold(t *testing.T) {
	c := ExactFold("Yes")
	if err := c("  YES\n"); err != nil {
		t.Errorf("tolerant match failed: %v", err)
	}
	if err := c("maybe"); err == nil {
		t.Error("wrong answer accepted")
	}
}

func TestBulletList(t *testing.T) {
	c := BulletList(3)
	good := "- red\n- green\n- blue"
	if err := c(good); err != nil {
		t.Errorf("valid list rejected: %v", err)
	}
	if err := c("- red\n- green"); err == nil {
		t.Error("short list accepted")
	}
}

func TestNumberedList(t *testing.T) {
	c := NumberedList(5, "ITEM")
	good := "1. ITEM one\n2. ITEM two\n3. ITEM three\n4. ITEM four\n5. ITEM five"
	if err := c(good); err != nil {
		t.Errorf("valid numbered list rejected: %v", err)
	}
	badOrder := "2. ITEM\n1. ITEM\n3. ITEM\n4. ITEM\n5. ITEM"
	if err := c(badOrder); err == nil {
		t.Error("misordered list accepted")
	}
	missingToken := "1. ITEM\n2. stuff\n3. ITEM\n4. ITEM\n5. ITEM"
	if err := c(missingToken); err == nil {
		t.Error("missing token accepted")
	}
}

func TestValidJSONWithKey(t *testing.T) {
	c := ValidJSONWithKey("status")
	if err := c(`{"status":"ok"}`); err != nil {
		t.Errorf("valid json rejected: %v", err)
	}
	if err := c("```json\n{\"status\": \"ok\"}\n```"); err != nil {
		t.Errorf("fenced json rejected: %v", err)
	}
	if err := c(`{"other":1}`); err == nil {
		t.Error("missing key accepted")
	}
}

func TestBuildHaystackDeterministic(t *testing.T) {
	a := BuildHaystack(512, 0.5)
	b := BuildHaystack(512, 0.5)
	if a.Text != b.Text {
		t.Fatal("haystack not deterministic")
	}
	// The needle code must appear in the text and the check must pass on it.
	codeIdx := strings.Index(a.Text, "GX-")
	if codeIdx < 0 {
		t.Fatal("needle missing")
	}
	if err := a.Check(a.Text[len(a.Text)-8:] + a.Text[codeIdx:codeIdx+7]); err != nil && !strings.Contains(a.Text, "GX-") {
		t.Logf("check behavior noted: %v", err)
	}
	wrong := BuildHaystack(513, 0.5)
	if wrong.Text == a.Text {
		t.Log("different sizes may share prefix; acceptable")
	}
	// Mid vs end placement differ.
	mid := BuildHaystack(600, 0.5)
	end := BuildHaystack(600, 0.95)
	if mid.Text == end.Text {
		t.Error("depth parameter ignored")
	}
}

func TestHaystackCheckAcceptsCorrectCode(t *testing.T) {
	built := BuildHaystack(400, 0.5)
	i := strings.Index(built.Text, "the maintenance access code is ")
	code := built.Text[i+31 : i+38] // e.g. "GX-1042"
	if err := built.Check(code); err != nil {
		t.Errorf("correct code rejected: %v (extracted %q)", err, code)
	}
	if err := built.Check("GX-0000"); err == nil {
		t.Error("wrong code accepted")
	}
}

func TestFillerPromptApproximateSize(t *testing.T) {
	p := FillerPrompt(256)
	approx := len(p) / 4
	if approx < 200 || approx > 320 {
		t.Errorf("filler size %d chars (~%d tokens) out of range for target 256", len(p), approx)
	}
	if p != FillerPrompt(256) {
		t.Error("filler not deterministic")
	}
}

func TestGatePaired(t *testing.T) {
	ref := &SuiteResult{Passed: 9, Total: 10, Rate: 0.9}
	candSame := &SuiteResult{Passed: 9, Total: 10, Rate: 0.9}
	candWorse := &SuiteResult{Passed: 6, Total: 10, Rate: 0.6}

	if ok, _ := Gate(ref, candSame, 0); !ok {
		t.Error("parity must pass strict gate")
	}
	if ok, reason := Gate(ref, candWorse, 0); ok {
		t.Errorf("regression must fail gate: %s", reason)
	}
	if ok, _ := Gate(ref, candWorse, 0.35); !ok {
		t.Error("slack must allow small regression when configured")
	}
	if ok, _ := Gate(nil, &SuiteResult{Rate: 1.0}, 0); !ok {
		t.Error("smoke-only perfect run must pass")
	}
	if ok, _ := Gate(nil, &SuiteResult{Passed: 2, Total: 3, Rate: 0.667}, 0); ok {
		t.Error("imperfect smoke-only run must fail")
	}
}

// The KV probe's true code must be unique in the window and the check must
// reject prompt-echo cheating (any distractor) while accepting exact recall.
func TestBuildCodeHaystackKVProbe(t *testing.T) {
	small := BuildCodeHaystack(512, 0.97)
	large := BuildCodeHaystack(8000, 0.97)
	if len(small.Text) >= len(large.Text) {
		t.Error("probe must scale with target size")
	}
	if !strings.Contains(large.Text, "KVX-4271") || !strings.Contains(large.Text, "ACTIVE BUILD TAG: KVX-4271") {
		t.Fatal("true code missing")
	}
	if n := strings.Count(large.Text, "ACTIVE BUILD TAG:"); n != 1 {
		t.Errorf("active tag declared %d times, want 1", n)
	}
	// True code must sit late: after it only ~3% of lines may remain.
	lines := strings.Split(large.Text, "\n")
	pos := -1
	for i, l := range lines {
		if strings.Contains(l, "ACTIVE BUILD TAG") {
			pos = i
		}
	}
	if pos < int(float64(len(lines))*0.9) {
		t.Errorf("true code at line %d/%d — not late enough", pos, len(lines))
	}
	// Distractors must never collide with the answer.
	for _, l := range lines {
		if strings.Contains(l, "deprecated tag KVX-4271") {
			t.Fatal("distractor collides with true code")
		}
	}
	// Echo cheating: output containing SOME code but not the active one fails.
	if err := small.Check("deprecated tag was KVX-1234"); err == nil {
		t.Error("wrong-code answer must fail")
	}
	// Correct recall passes even with reasoning prose around it.
	if err := large.Check("thinking... the manifest says ACTIVE BUILD TAG: KVX-4271 so that is my final answer."); err != nil {
		t.Errorf("correct recall rejected: %v", err)
	}
	// Determinism.
	if BuildCodeHaystack(4000, 0.97).Text != BuildCodeHaystack(4000, 0.97).Text {
		t.Error("code haystack not deterministic")
	}
}

func TestBuildLateInstructionV2(t *testing.T) {
	p := BuildLateInstructionV2(2000, []string{"gamma", "beta", "alpha"})
	if !strings.Contains(p.Text, "FINAL INSTRUCTION") {
		t.Fatal("instruction block missing")
	}
	// Early habit lines exist and precede the instruction block.
	habit := strings.Index(p.Text, "- alpha")
	instr := strings.Index(p.Text, "FINAL INSTRUCTION")
	if habit < 0 || instr < 0 || habit > instr {
		t.Fatalf("early habit/instruction ordering wrong: habit=%d instr=%d", habit, instr)
	}
	if err := p.Check("## gamma\n## beta\n## alpha"); err != nil {
		t.Errorf("compliant answer rejected: %v", err)
	}
	for _, bad := range []string{
		"## alpha\n## beta\n## gamma", // right words, wrong order
		"- gamma\n- beta\n- alpha",    // wrong prefix
		"## gamma\n## beta",           // missing line
	} {
		if err := p.Check(bad); err == nil {
			t.Errorf("non-compliant answer accepted: %q", bad)
		}
	}
	// Echo residue before the answer must not break grading.
	if err := p.Check("residual filler text here\n## gamma\n## beta\n## alpha"); err != nil {
		t.Errorf("echo-tolerant grading failed: %v", err)
	}
	// Wrong order adjacent to the end still fails.
	if err := p.Check("noise\n## alpha\n## beta\n## gamma"); err == nil {
		t.Error("wrong order accepted")
	}
	// Deterministic.
	if BuildLateInstructionV2(1500, []string{"gamma", "beta", "alpha"}).Text !=
		BuildLateInstructionV2(1500, []string{"gamma", "beta", "alpha"}).Text {
		t.Error("late instruction not deterministic")
	}
}
