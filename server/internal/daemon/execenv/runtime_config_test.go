package execenv

import (
	"testing"
)

func TestScopeIndexForBudget_UnderLimit(t *testing.T) {
	entries := []DocumentIndexEntry{
		{Path: "a.md"},
		{Path: "b.md"},
	}
	result, dropped := scopeIndexForBudget(entries, "MyProject", 200)
	if len(result) != 2 || dropped != 0 {
		t.Fatalf("expected 2 entries and 0 dropped, got %d and %d", len(result), dropped)
	}
}

func TestScopeIndexForBudget_OverLimitNoProject(t *testing.T) {
	entries := make([]DocumentIndexEntry, 250)
	for i := range entries {
		entries[i] = DocumentIndexEntry{Path: "doc/" + string(rune('a'+i%26)) + ".md"}
	}
	result, dropped := scopeIndexForBudget(entries, "", 200)
	if len(result) != 200 {
		t.Fatalf("expected 200 entries, got %d", len(result))
	}
	if dropped != 50 {
		t.Fatalf("expected 50 dropped, got %d", dropped)
	}
}

func TestScopeIndexForBudget_OverLimitWithProject(t *testing.T) {
	entries := make([]DocumentIndexEntry, 0, 210)
	// 10 project-scoped entries
	for i := 0; i < 10; i++ {
		entries = append(entries, DocumentIndexEntry{Path: "myproject/doc" + string(rune('a'+i)) + ".md"})
	}
	// 200 other entries
	for i := 0; i < 200; i++ {
		entries = append(entries, DocumentIndexEntry{Path: "other/doc" + string(rune('a'+i%26)) + ".md"})
	}

	result, dropped := scopeIndexForBudget(entries, "myproject", 200)

	// All 10 project entries should be included
	projectCount := 0
	for _, e := range result {
		if len(e.Path) > 10 && e.Path[:10] == "myproject/" {
			projectCount++
		}
	}
	if projectCount != 10 {
		t.Fatalf("expected 10 project entries, got %d", projectCount)
	}
	if len(result) != 200 {
		t.Fatalf("expected 200 entries total, got %d", len(result))
	}
	if dropped != 10 {
		t.Fatalf("expected 10 dropped, got %d", dropped)
	}
}

func TestScopeIndexForBudget_ResultIsSorted(t *testing.T) {
	entries := []DocumentIndexEntry{
		{Path: "zzz/doc.md"},
		{Path: "aaa/doc.md"},
		{Path: "myproj/doc.md"},
	}
	// Under limit — should return as-is (already passes through)
	result, _ := scopeIndexForBudget(entries, "myproj", 2)
	// Over limit — should be sorted
	for i := 1; i < len(result); i++ {
		if result[i].Path < result[i-1].Path {
			t.Fatalf("result not sorted: %s < %s", result[i].Path, result[i-1].Path)
		}
	}
}
