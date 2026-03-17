package uicons

import (
	"testing"
)

func TestPokemonCandidates(t *testing.T) {
	candidates := pokemonCandidates("png", 25, 0, 0, 0, 0, 0, false, 0)
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate for pokemonID=25")
	}
	if candidates[0] != "25.png" {
		t.Errorf("first candidate = %q, want %q", candidates[0], "25.png")
	}
}

func TestPokemonCandidatesWithForm(t *testing.T) {
	candidates := pokemonCandidates("png", 25, 61, 0, 0, 0, 0, false, 0)
	if len(candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d", len(candidates))
	}
	// First candidate should include the form suffix
	if candidates[0] != "25_f61.png" {
		t.Errorf("first candidate = %q, want %q", candidates[0], "25_f61.png")
	}
	// Last candidate should be the fallback without form
	last := candidates[len(candidates)-1]
	if last != "25.png" {
		t.Errorf("last candidate = %q, want %q", last, "25.png")
	}
}

func TestPokemonCandidatesZeroID(t *testing.T) {
	candidates := pokemonCandidates("png", 0, 0, 0, 0, 0, 0, false, 0)
	if len(candidates) != 0 {
		t.Errorf("expected no candidates for pokemonID=0, got %d", len(candidates))
	}
}

func TestEggCandidates(t *testing.T) {
	candidates := eggCandidates("png", 5, false, false)
	if len(candidates) != 1 || candidates[0] != "5.png" {
		t.Errorf("eggCandidates(5, false, false) = %v, want [5.png]", candidates)
	}
}

func TestEggCandidatesHatched(t *testing.T) {
	candidates := eggCandidates("png", 3, true, false)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0] != "3_h.png" {
		t.Errorf("first candidate = %q, want %q", candidates[0], "3_h.png")
	}
	if candidates[1] != "3.png" {
		t.Errorf("second candidate = %q, want %q", candidates[1], "3.png")
	}
}

func TestEggCandidatesZeroLevel(t *testing.T) {
	candidates := eggCandidates("png", 0, false, false)
	if len(candidates) != 0 {
		t.Errorf("expected no candidates for level=0, got %d", len(candidates))
	}
}

func TestRewardCandidates(t *testing.T) {
	candidates := rewardCandidates("png", 1, 0)
	if len(candidates) != 1 || candidates[0] != "1.png" {
		t.Errorf("rewardCandidates(1, 0) = %v, want [1.png]", candidates)
	}
}

func TestRewardCandidatesWithAmount(t *testing.T) {
	candidates := rewardCandidates("png", 3, 500)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0] != "3_a500.png" {
		t.Errorf("first candidate = %q, want %q", candidates[0], "3_a500.png")
	}
	if candidates[1] != "3.png" {
		t.Errorf("second candidate = %q, want %q", candidates[1], "3.png")
	}
}

func TestRewardCandidatesZeroID(t *testing.T) {
	candidates := rewardCandidates("png", 0, 0)
	if len(candidates) != 0 {
		t.Errorf("expected no candidates for id=0, got %d", len(candidates))
	}
}

func TestCachedClientNil(t *testing.T) {
	if client := CachedClient("", "png"); client != nil {
		t.Error("expected nil for empty baseURL")
	}
}

func TestCachedClientReuse(t *testing.T) {
	c1 := CachedClient("https://example.com/icons", "png")
	c2 := CachedClient("https://example.com/icons", "png")
	if c1 != c2 {
		t.Error("expected same client instance for same baseURL+imageType")
	}
}
