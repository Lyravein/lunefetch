package components

import "testing"

func TestCategoryItemsUseUIAllAndCanonicalOrder(t *testing.T) {
	want := []string{"All", "Compressed", "Documents", "Media", "Programs", "Other"}
	if len(categoryItems) != len(want) {
		t.Fatalf("category item count = %d, want %d", len(categoryItems), len(want))
	}
	for i, label := range want {
		if categoryItems[i].Label != label {
			t.Errorf("categoryItems[%d] = %q, want %q", i, categoryItems[i].Label, label)
		}
	}
}
