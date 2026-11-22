package season

import (
	"strings"

	"github.com/shrujancheruku/issho/backend/internal/people"
)

// Member is one scripted person. Every detail is invented: the names are made
// up and unlike any real person's or any word a log line holds (the log search
// looks for them), the emails are on example.com, and the account ids are
// fixed so two runs make the same accounts.
type Member struct {
	Index     int
	ID        string
	Email     string
	Name      string
	FirstName string
	LastName  string
	Gender    people.Gender
	Languages []string
	WomenOnly bool
	// Inviter is the member whose join link admits this one, -1 for the
	// founder, who is admitted by the seed.
	Inviter int
	// Admitted is whether the script gets them admitted. The last few never
	// are, and try to sign up anyway.
	Admitted bool
	// Plans are the plans they sign up to: bit 0 the lanterns, bit 1 the sunrise.
	plans int
}

var (
	firstNames = []string{
		"Anvith", "Belenor", "Caspian", "Dalwyn", "Elowen", "Fenwick", "Gwynlyn", "Hesperin", "Ilmarin", "Jorvald",
		"Kestrin", "Lyrawen", "Maelis", "Nerival", "Orsolen", "Pellamy", "Quillon", "Rowenna", "Sorrelle", "Tamsyn",
	}
	lastNames = []string{
		"Ashgrove", "Brightwater", "Cindermoor", "Dunmere", "Evenfall", "Fairweather", "Glenmoor", "Hollowbrook",
		"Ivywood", "Juniperlane", "Kestrelwood", "Larkspur", "Mistholme", "Northcott", "Oakhaven", "Pebblecombe",
		"Quillfeather", "Ravenscroft", "Stonebridge", "Thistledown",
	}
	// Most speak English and Vietnamese, as the city's people do; a few speak
	// one or something else, so language is a real constraint on a group.
	languages = [][]string{
		{"en", "vi"}, {"en", "vi"}, {"vi"}, {"en"}, {"en", "vi"}, {"en", "vi"}, {"vi"}, {"en", "fr"}, {"en", "vi"}, {"en", "vi"},
		{"vi"}, {"en"}, {"en", "vi"}, {"en", "vi"}, {"en", "ko"}, {"en", "vi"}, {"en", "vi"}, {"en", "vi"}, {"en"}, {"vi"},
	}
	genders = []people.Gender{
		people.GenderMan, people.GenderWoman, people.GenderNonBinary, people.GenderMan, people.GenderWoman,
		people.GenderWoman, people.GenderMan, people.GenderPreferNotToSay, people.GenderMan, people.GenderWoman,
	}
	// inviters: member 0 is the founder; each member brings in at most three
	// (the limit per member), and 17 to 19 are never admitted.
	inviters = []int{-1, 0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3, 4, 4, 4, 5, -2, -2, -2} //policylint:ignore a scripted number of the season, not that rule
	// womenOnly: two women ask for groups of women and non-binary people only.
	womenOnly = map[int]bool{4: true, 9: true} //policylint:ignore a scripted number of the season, not that rule
)

// castOf is the scripted cast: 17 admitted members in a tree of join links
// under the founder, and three who never are.
func castOf() []Member {
	cast := make([]Member, len(firstNames))
	for i := range cast {
		g := genders[i%len(genders)]
		cast[i] = Member{
			Index:     i,
			ID:        accountID(i),
			Email:     strings.ToLower(firstNames[i]+"."+lastNames[i]) + "@example.com",
			Name:      firstNames[i] + " " + lastNames[i],
			FirstName: firstNames[i],
			LastName:  lastNames[i],
			Gender:    g,
			Languages: languages[i],
			WomenOnly: womenOnly[i] && (g == people.GenderWoman || g == people.GenderNonBinary),
			Inviter:   inviters[i],
			Admitted:  inviters[i] >= -1,
			plans:     []int{3, 1, 1, 2}[i%4], //policylint:ignore a scripted number of the season, not that rule
		}
	}
	return cast
}

// accountID is member n's account id: fixed, so a run makes the same accounts.
func accountID(n int) string {
	const hex = "0123456789abcdef"
	s := []byte("5ea50000-0000-4000-8000-000000000000")
	s[len(s)-1] = hex[n%16]
	s[len(s)-2] = hex[n/16]
	return string(s)
}
