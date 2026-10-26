package profile

import (
	"context"
	"errors"
	"maps"
	"slices"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) GetProfile(ctx context.Context, _ *connect.Request[isshov1.GetProfileRequest]) (*connect.Response[isshov1.GetProfileResponse], error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.Svc.Get(ctx, c.UserID)
	if errors.Is(err, ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.GetProfileResponse{Profile: toProto(p), CurrentConductVersion: ConductVersion}), nil
}

func (h *Handler) SaveProfile(ctx context.Context, req *connect.Request[isshov1.SaveProfileRequest]) (*connect.Response[isshov1.SaveProfileResponse], error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.Profile == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("profile: required"))
	}
	in, err := fromProto(req.Msg.Profile)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	p, err := h.Svc.Save(ctx, c.UserID, in)
	switch {
	case IsInvalid(err):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, ErrNoAccount):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case err != nil:
		return nil, err
	}
	return connect.NewResponse(&isshov1.SaveProfileResponse{Profile: toProto(p)}), nil
}

var (
	genders = map[isshov1.Gender]string{
		isshov1.Gender_GENDER_WOMAN:             "woman",
		isshov1.Gender_GENDER_MAN:               "man",
		isshov1.Gender_GENDER_NONBINARY:         "nonbinary",
		isshov1.Gender_GENDER_PREFER_NOT_TO_SAY: "unspecified",
	}
	sizes = map[isshov1.GroupSize]string{
		isshov1.GroupSize_GROUP_SIZE_SMALL:  "small",
		isshov1.GroupSize_GROUP_SIZE_MEDIUM: "medium",
		isshov1.GroupSize_GROUP_SIZE_LARGE:  "large",
	}
	timeBlocks = map[isshov1.TimeBlock]string{
		isshov1.TimeBlock_TIME_BLOCK_MORNING:   "morning",
		isshov1.TimeBlock_TIME_BLOCK_AFTERNOON: "afternoon",
		isshov1.TimeBlock_TIME_BLOCK_EVENING:   "evening",
	}
	// Interest levels map to app.user_activity_prefs.weight.
	interests = map[isshov1.Interest]int{
		isshov1.Interest_INTEREST_OPEN_TO_TRY: 0,
		isshov1.Interest_INTEREST_LIKE:        1,
		isshov1.Interest_INTEREST_LOVE:        2,
	}
)

func reverse[K, V comparable](m map[K]V) map[V]K {
	r := make(map[V]K, len(m))
	for k, v := range m {
		r[v] = k
	}
	return r
}

var (
	gendersPB   = reverse(genders)
	sizesPB     = reverse(sizes)
	blocksPB    = reverse(timeBlocks)
	interestsPB = reverse(interests)
)

// fromProto converts the request; unknown enum values become "" and are
// rejected by validate, except for the lists, which are checked here.
func fromProto(m *isshov1.Profile) (Profile, error) {
	p := Profile{
		DisplayName:       m.DisplayName,
		BirthYear:         int(m.BirthYear),
		Gender:            genders[m.Gender],
		SameGenderOnly:    m.SameGenderOnly,
		HomeAreaID:        m.HomeAreaId,
		MaxTravelMin:      int(m.MaxTravelMin),
		PrimaryLanguage:   m.PrimaryLanguage,
		SecondaryLanguage: m.SecondaryLanguage,
		SizePref:          sizes[m.GroupSize],
		Energy:            int(m.Energy),
		Budget:            int(m.Budget),
		ConductVersion:    int(m.ConductVersion),
		Prefs:             map[string]int{},
	}
	for _, l := range m.SpokenLanguages {
		p.Languages = append(p.Languages, Language{Code: l.Code, Fluent: l.Fluent, Level: int(l.Level)})
	}
	for _, s := range m.Availability {
		b, ok := timeBlocks[s.Block]
		if !ok {
			return Profile{}, invalid("availability", "unknown time block")
		}
		p.Availability = append(p.Availability, Slot{Weekday: int(s.Weekday), Block: b})
	}
	for _, a := range m.Activities {
		w, ok := interests[a.Interest]
		if !ok {
			return Profile{}, invalid("activities", "unknown interest for %q", a.ActivityId)
		}
		if _, dup := p.Prefs[a.ActivityId]; dup {
			return Profile{}, invalid("activities", "%q listed twice", a.ActivityId)
		}
		p.Prefs[a.ActivityId] = w
	}
	return p, nil
}

func toProto(p Profile) *isshov1.Profile {
	m := &isshov1.Profile{
		DisplayName:       p.DisplayName,
		BirthYear:         int32(p.BirthYear),
		Gender:            gendersPB[p.Gender],
		SameGenderOnly:    p.SameGenderOnly,
		CityId:            p.CityID,
		HomeAreaId:        p.HomeAreaID,
		MaxTravelMin:      int32(p.MaxTravelMin),
		PrimaryLanguage:   p.PrimaryLanguage,
		SecondaryLanguage: p.SecondaryLanguage,
		GroupSize:         sizesPB[p.SizePref],
		Energy:            int32(p.Energy),
		Budget:            int32(p.Budget),
		ConductVersion:    int32(p.ConductVersion),
	}
	for _, l := range p.Languages {
		m.SpokenLanguages = append(m.SpokenLanguages, &isshov1.SpokenLanguage{Code: l.Code, Fluent: l.Fluent, Level: int32(l.Level)})
	}
	for _, s := range p.Availability {
		m.Availability = append(m.Availability, &isshov1.AvailabilitySlot{Weekday: int32(s.Weekday), Block: blocksPB[s.Block]})
	}
	for _, id := range slices.Sorted(maps.Keys(p.Prefs)) {
		m.Activities = append(m.Activities, &isshov1.ActivityPreference{ActivityId: id, Interest: interestsPB[p.Prefs[id]]})
	}
	return m
}
