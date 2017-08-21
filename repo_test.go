package git

import "testing"

func TestGetRefs(t *testing.T) {

	expected := []string{"v1.0.0", "v1.2.1", "v2.0.0", "v3.0.0", "v3.1.0", "v3.10.1"}

	for _, p := range []string{
		"testdata/test_loose_refs.git",
		"testdata/test_packed_refs.git",
		"testdata/test_mixed_refs.git",
	} {
		r, err := OpenRepository(p)
		if err != nil {
			t.Fatal(err)
		}
		tags, err := r.GetTags()
		if err != nil {
			t.Fatal(err)
		}

		if len(expected) != len(tags) {
			t.Fatalf("wrong number of tags returned - expected [%d] got [%d]", len(expected), len(tags))
		}

		for i, v := range tags {
			if expected[i] != v {
				t.Fatalf("incorrect tag - expected [%s] got [%s]", expected[i], v)
			}
		}
	}

}
