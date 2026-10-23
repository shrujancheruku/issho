package walks

import "testing"

// Pairs from real runs (2026-10-22/23): the model's name, Google's English
// name for what Text Search returned, and whether they're the same place.
func TestSameName(t *testing.T) {
	koenji := Request{City: "Tokyo", Area: "Koenji"}
	kichijoji := Request{City: "Tokyo", Area: "Kichijoji"}
	nakameguro := Request{City: "Tokyo", Area: "Naka-Meguro"}
	for _, tc := range []struct {
		req           Request
		model, google string
		want          bool
	}{
		// Made-up or wrong places that Text Search mapped onto real ones.
		{koenji, "Hase-ji Temple", "Kōenji Temple", false},
		{koenji, "Midori Kumo Shrine", "Kōenji Hikawa Shrine", false},
		{koenji, "Cafe Zzyzx Blue Lantern", "HATTIFNATT ハティフナット -高円寺のおうち-", false},
		{koenji, "Gion", "Junjō Shōtengai -Pure Heart Shopping Street-", false},
		// The same places, named differently.
		{koenji, "Koenji Station South Exit", "Kōenji Station", true},
		{koenji, "Shukuhōzan Kōen-ji Temple", "Kōenji Temple", true},
		{koenji, "Koenji Hikawa Shrine & Kishō Shrine", "Kōenji Hikawa Shrine", true},
		{koenji, "Sotai-in Temple", "Sōtai-in", true},
		{kichijoji, "Harmonica Yokocho", "Harmonica Yokocyo", true},
		{kichijoji, "Kichijoji Satou", "Satou Kichijoji", true},
		{kichijoji, "Sun Road Shopping Street", "Kichijoji SUNROAD shopping district", true},
		{kichijoji, "Inokashira Onshi Park", "Inokashira Park", true},
		{nakameguro, "Naka-Meguro Station Main Exit", "Naka-meguro Sta.", true},
		{nakameguro, "Meguro River Promenade", "Meguro River Cherry Blossoms Promenade", true},
		{nakameguro, "Spring Valley Brewery Tokyo", "Spring Valley Brewery Tokyo", true},
	} {
		if got := sameName(tc.model, tc.google, tc.req); got != tc.want {
			t.Errorf("sameName(%q, %q) = %v, want %v", tc.model, tc.google, got, tc.want)
		}
	}
}
