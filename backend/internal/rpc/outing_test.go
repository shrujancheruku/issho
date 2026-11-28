package rpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

type outingRig struct {
	*signupRig
	outings isshov1connect.OutingServiceClient
}

func newOutingRig(t *testing.T) *outingRig {
	t.Helper()
	r := newSignupRig(t)
	return &outingRig{signupRig: r, outings: isshov1connect.NewOutingServiceClient(r.srv.Client(), r.srv.URL)}
}

// named is an admitted, signed-in account with a profile under its own name.
func (r *outingRig) named(t *testing.T, id, email, name string) string {
	t.Helper()
	tok := r.signedIn(t, id, email)
	r.admit(t, email)
	req := goodSave()
	req.Name = name
	if _, err := r.save(tok, req); err != nil {
		t.Fatal(err)
	}
	return tok
}

func (r *outingRig) list(token string) (*isshov1.ListMyOutingsResponse, error) {
	res, err := r.outings.ListMyOutings(context.Background(), authed(token, &isshov1.ListMyOutingsRequest{}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

const (
	third  = "33333333-3333-4333-8333-333333333333"
	fourth = "44444444-4444-4444-8444-444444444444"
)

// Three people sign up to the lantern plan for one evening and matching forms
// their group.
func (r *outingRig) formedGroup(t *testing.T) (adaTok, graceTok, thirdTok string) {
	t.Helper()
	adaTok = r.named(t, ada, "ada@example.com", "Ada Lovelace")
	graceTok = r.named(t, grace, "grace@example.com", "Grace Hopper")
	thirdTok = r.named(t, third, "third@example.com", "Katherine Johnson")
	for _, tok := range []string{adaTok, graceTok, thirdTok} {
		if _, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening")); err != nil {
			t.Fatal(err)
		}
	}
	if formed, err := r.forming.Form(context.Background(), lanterns); err != nil || formed != 1 {
		t.Fatalf("Form = %d, %v; want 1 outing", formed, err)
	}
	return adaTok, graceTok, thirdTok
}

// An invited person sees the date, the time and the languages the group shares,
// when the invitation went out and when its 48 hours end, and never a name.
func TestAnInvitedPersonSeesTheDateTheTimeAndTheSharedLanguagesNeverNames(t *testing.T) {
	r := newOutingRig(t)
	adaTok, _, _ := r.formedGroup(t)

	got, err := r.list(adaTok)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.GetOutings()) != 1 {
		t.Fatalf("outings = %v, want 1", got.GetOutings())
	}
	o := got.GetOutings()[0]
	sent := accountStart
	if o.GetPlanId() != lanterns || o.GetDate() != "2026-01-09" || o.GetPart() != "evening" || o.GetStartsAt() != "17:00" ||
		strings.Join(o.GetLanguages(), ",") != "en,vi" || o.GetSeatState() != isshov1.SeatState_SEAT_STATE_INVITED ||
		!o.GetInvitedAt().AsTime().Equal(sent) || !o.GetInvitationEndsAt().AsTime().Equal(sent.Add(48*time.Hour)) {
		t.Fatalf("outing = %v", o)
	}
	// Nothing of anyone else is in what the wire carries: no name, no account.
	wire, err := protojson.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []string{"Grace", "Hopper", "Katherine", "Johnson", grace, third} {
		if strings.Contains(string(wire), other) {
			t.Fatalf("the response names %q: %s", other, wire)
		}
	}
}

// No RPC returns an outing to someone with no seat in it: a member who isn't in
// the group lists none, and nobody can name an outing to ask for it (the
// request has no field for one).
func TestAMemberWithNoSeatInAnOutingIsNeverReturnedIt(t *testing.T) {
	r := newOutingRig(t)
	r.formedGroup(t)
	outsider := r.named(t, fourth, "fourth@example.com", "Mary Jackson")

	got, err := r.list(outsider)

	if err != nil || len(got.GetOutings()) != 0 {
		t.Fatalf("an outsider lists %v, %v; want none", got.GetOutings(), err)
	}
}

// Listing outings needs a signed-in caller.
func TestTheOutingRPCNeedsASignedInCaller(t *testing.T) {
	r := newOutingRig(t)
	_, err := r.list("")
	wantCode(t, err, connect.CodeUnauthenticated)
}

func (r *outingRig) answer(token, outing string, a isshov1.InvitationAnswer) (*isshov1.Outing, error) {
	res, err := r.outings.AnswerInvitation(context.Background(), authed(token, &isshov1.AnswerInvitationRequest{OutingId: outing, Answer: a}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetOuting(), nil
}

func (r *outingRig) cancel(token, outing string) (*isshov1.Outing, error) {
	res, err := r.outings.CancelSeat(context.Background(), authed(token, &isshov1.CancelSeatRequest{OutingId: outing}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetOuting(), nil
}

// outingOf is the caller's one outing.
func (r *outingRig) outingOf(t *testing.T, token string) *isshov1.Outing {
	t.Helper()
	got, err := r.list(token)
	if err != nil || len(got.GetOutings()) != 1 {
		t.Fatalf("list = %v, %v; want one outing", got, err)
	}
	return got.GetOutings()[0]
}

// An invited person answers yes or no over the API and the outing comes back as
// they see it; a yes waits for the outing to be set, so it is not going yet.
func TestAnInvitedPersonAnswersYesOrNoOverTheAPI(t *testing.T) {
	r := newOutingRig(t)
	adaTok, graceTok, _ := r.formedGroup(t)
	outing := r.outingOf(t, adaTok).GetId()

	yes, err := r.answer(adaTok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	if err != nil {
		t.Fatal(err)
	}
	no, err := r.answer(graceTok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_NO)
	if err != nil {
		t.Fatal(err)
	}

	if yes.GetSeatState() != isshov1.SeatState_SEAT_STATE_ACCEPTED || yes.GetOutingState() != isshov1.OutingState_OUTING_STATE_OFFERED || yes.GetYesCount() != 1 {
		t.Errorf("after a yes: %v", yes)
	}
	if no.GetSeatState() != isshov1.SeatState_SEAT_STATE_DECLINED || no.GetYesCount() != 1 {
		t.Errorf("after a no: %v", no)
	}
}

// After the 48 hours the invitation is closed, over the API too; an answer that
// is neither yes nor no is refused; an outing the caller has no seat in is not
// found, whether or not it exists.
func TestAnAnswerAfterTheFortyEightHoursOrWithoutASeatIsRefusedOverTheAPI(t *testing.T) {
	r := newOutingRig(t)
	adaTok, _, _ := r.formedGroup(t)
	outing := r.outingOf(t, adaTok).GetId()
	outsider := r.named(t, fourth, "fourth@example.com", "Mary Jackson")

	_, unspecified := r.answer(adaTok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_UNSPECIFIED)
	_, stranger := r.answer(outsider, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	_, missing := r.answer(adaTok, "not-an-outing", isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	r.clk.Advance(48 * time.Hour)
	adaTok = r.signedIn(t, ada, "ada@example.com") // the first token expired with the hour
	_, late := r.answer(adaTok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)

	wantCode(t, unspecified, connect.CodeInvalidArgument)
	wantCode(t, stranger, connect.CodeNotFound)
	wantCode(t, missing, connect.CodeNotFound)
	wantCode(t, late, connect.CodeFailedPrecondition)
}

// What the group shows of itself, on the wire: an invited person sees the count
// of yeses and no name at all; once the outing is set, a person going sees the
// first names of the others going, not their own and not the rest of a name, and
// still no account id.
func TestTheWireShowsACountToTheInvitedAndFirstNamesToThoseGoing(t *testing.T) {
	r := newOutingRig(t)
	adaTok, graceTok, thirdTok := r.formedGroup(t)
	outing := r.outingOf(t, adaTok).GetId()
	r.answer(adaTok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	r.answer(graceTok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)

	waiting, err := r.list(adaTok)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := protojson.Marshal(waiting)
	for _, name := range []string{"Grace", "Katherine", "Hopper", "Johnson", grace, third} {
		if strings.Contains(string(wire), name) {
			t.Fatalf("before the outing is set, the wire names %q: %s", name, wire)
		}
	}
	if waiting.GetOutings()[0].GetYesCount() != 2 {
		t.Fatalf("yes count = %d, want 2", waiting.GetOutings()[0].GetYesCount())
	}

	setBy, err := r.answer(thirdTok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	if err != nil {
		t.Fatal(err)
	}
	ada := r.outingOf(t, adaTok)

	if setBy.GetOutingState() != isshov1.OutingState_OUTING_STATE_SET || setBy.GetSeatState() != isshov1.SeatState_SEAT_STATE_GOING {
		t.Errorf("the third yes: %v", setBy)
	}
	if got := strings.Join(ada.GetOthersGoing(), ","); got != "Grace,Katherine" {
		t.Errorf("Ada sees %q going, want Grace,Katherine", got)
	}
	wire, _ = protojson.Marshal(&isshov1.ListMyOutingsResponse{Outings: []*isshov1.Outing{ada}})
	for _, leak := range []string{"Hopper", "Johnson", "Lovelace", grace, third} {
		if strings.Contains(string(wire), leak) {
			t.Fatalf("the wire carries %q: %s", leak, wire)
		}
	}
}

// A person going cancels over the API, and the seat comes back cancelled; one
// who isn't going can't, and the outing isn't shown to those with no seat.
func TestAPersonGoingCancelsOverTheAPI(t *testing.T) {
	r := newOutingRig(t)
	adaTok, graceTok, thirdTok := r.formedGroup(t)
	outing := r.outingOf(t, adaTok).GetId()
	_, early := r.cancel(adaTok, outing)
	for _, tok := range []string{adaTok, graceTok, thirdTok} {
		r.answer(tok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	}

	cancelled, err := r.cancel(adaTok, outing)

	wantCode(t, early, connect.CodeFailedPrecondition)
	if err != nil || cancelled.GetSeatState() != isshov1.SeatState_SEAT_STATE_CANCELLED {
		t.Fatalf("cancel = %v, %v; want a cancelled seat", cancelled, err)
	}
	outsider := r.named(t, fourth, "fourth@example.com", "Mary Jackson")
	_, err = r.cancel(outsider, outing)
	wantCode(t, err, connect.CodeNotFound)
}

// The new RPCs need a signed-in member.
func TestAnsweringAndCancellingNeedASignedInCaller(t *testing.T) {
	r := newOutingRig(t)
	_, answer := r.answer("", "x", isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	_, cancel := r.cancel("", "x")
	wantCode(t, answer, connect.CodeUnauthenticated)
	wantCode(t, cancel, connect.CodeUnauthenticated)
}

// The descriptor walk in TestNoRPCReturnsAnotherPersonsEmail finds a field that
// could hold an email; this reads what the outing RPCs actually send. Three
// people with a known email and a full name go through every outing call (list,
// yes, set, list again going, cancel), and in none of the responses is there an
// email, an account id or more of a name than the first.
func TestTheOutingRPCsNeverSendAnotherPersonsEmailIdOrSurname(t *testing.T) {
	r := newOutingRig(t)
	adaTok, graceTok, thirdTok := r.formedGroup(t)
	outing := r.outingOf(t, adaTok).GetId()
	var sent []string
	record := func(m proto.Message) {
		b, err := protojson.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		sent = append(sent, string(b))
	}
	for _, tok := range []string{adaTok, graceTok, thirdTok} {
		got, err := r.answer(tok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
		if err != nil {
			t.Fatal(err)
		}
		record(got)
	}
	for _, tok := range []string{adaTok, graceTok, thirdTok} {
		list, err := r.list(tok)
		if err != nil {
			t.Fatal(err)
		}
		record(list)
	}
	cancelled, err := r.cancel(graceTok, outing)
	if err != nil {
		t.Fatal(err)
	}
	record(cancelled)
	// On the day and after: check-in, then feedback's form, an answer with a
	// pick, and a "no", each as every person sends them.
	adaTok, _, thirdTok = r.at(t, outingStart)
	for _, tok := range []string{adaTok, thirdTok} {
		in, err := r.checkIn(tok, outing)
		if err != nil {
			t.Fatal(err)
		}
		record(in)
	}
	adaTok, _, thirdTok = r.at(t, outingEnd)
	for _, tok := range []string{adaTok, thirdTok} {
		form, err := r.feedback(tok, outing)
		if err != nil {
			t.Fatal(err)
		}
		record(form)
		req := &isshov1.SaveFeedbackRequest{OutingId: outing, Went: true}
		for _, c := range form.GetOthers() {
			req.MeetAgain = append(req.MeetAgain, c.GetHandle())
		}
		saved, err := r.saveFeedback(tok, req)
		if err != nil {
			t.Fatal(err)
		}
		record(saved)
		no, err := r.saveFeedback(tok, &isshov1.SaveFeedbackRequest{OutingId: outing, Went: false})
		if err != nil {
			t.Fatal(err)
		}
		record(no)
	}

	all := strings.Join(sent, "\n")
	for _, leak := range []string{"@", "example.com", ada, grace, third, "Lovelace", "Hopper", "Johnson"} {
		if strings.Contains(all, leak) {
			t.Errorf("an outing response carries %q: %s", leak, all)
		}
	}
	// The first names are what is meant to be shown, so the scan can see them.
	if !strings.Contains(all, "Grace") || !strings.Contains(all, "Katherine") {
		t.Errorf("no first names in the responses, so the scan proved nothing: %s", all)
	}
}

// The plan runs 17:00 to 20:30 on the city's clock (UTC+7).
var (
	outingStart = time.Date(2026, 1, 9, 10, 0, 0, 0, time.UTC)
	outingEnd   = time.Date(2026, 1, 9, 13, 30, 0, 0, time.UTC)
)

func (r *outingRig) checkIn(token, outing string) (*isshov1.Outing, error) {
	res, err := r.outings.CheckIn(context.Background(), authed(token, &isshov1.CheckInRequest{OutingId: outing}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetOuting(), nil
}

func (r *outingRig) feedback(token, outing string) (*isshov1.Feedback, error) {
	res, err := r.outings.GetFeedback(context.Background(), authed(token, &isshov1.GetFeedbackRequest{OutingId: outing}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetFeedback(), nil
}

func (r *outingRig) saveFeedback(token string, req *isshov1.SaveFeedbackRequest) (*isshov1.Feedback, error) {
	res, err := r.outings.SaveFeedback(context.Background(), authed(token, req))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetFeedback(), nil
}

// A set outing whose three people are all going, with the clock at the moment
// they said so.
func (r *outingRig) goingGroup(t *testing.T) (adaTok, graceTok, thirdTok, outing string) {
	t.Helper()
	adaTok, graceTok, thirdTok = r.formedGroup(t)
	outing = r.outingOf(t, adaTok).GetId()
	for _, tok := range []string{adaTok, graceTok, thirdTok} {
		if _, err := r.answer(tok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES); err != nil {
			t.Fatal(err)
		}
	}
	return adaTok, graceTok, thirdTok, outing
}

// at moves the clock and signs the three in again, since a token lasts an hour.
func (r *outingRig) at(t *testing.T, now time.Time) (adaTok, graceTok, thirdTok string) {
	t.Helper()
	r.clk.Set(now)
	return r.signedIn(t, ada, "ada@example.com"), r.signedIn(t, grace, "grace@example.com"), r.signedIn(t, third, "third@example.com")
}

// Check-in over the API opens 30 minutes before the start, shows who has
// checked in by first name to those going, and is refused outside its window.
func TestCheckInOverTheAPIShowsWhoHasCheckedIn(t *testing.T) {
	r := newOutingRig(t)
	_, _, _, outing := r.goingGroup(t)

	earlyTok, _, _ := r.at(t, outingStart.Add(-31*time.Minute))
	_, early := r.checkIn(earlyTok, outing)
	adaTok, graceTok, thirdTok := r.at(t, outingStart.Add(-30*time.Minute))
	_, err := r.checkIn(graceTok, outing)
	if err != nil {
		t.Fatal(err)
	}
	mine, err := r.checkIn(adaTok, outing)
	if err != nil {
		t.Fatal(err)
	}
	katherine, err := r.list(thirdTok)
	if err != nil {
		t.Fatal(err)
	}
	_, _, thirdTok = r.at(t, outingEnd)
	_, late := r.checkIn(thirdTok, outing)

	wantCode(t, early, connect.CodeFailedPrecondition)
	wantCode(t, late, connect.CodeFailedPrecondition)
	if !mine.GetCheckedIn() || !mine.GetCheckedInAt().AsTime().Equal(outingStart.Add(-30*time.Minute)) {
		t.Errorf("after checking in: %v", mine)
	}
	if got := strings.Join(mine.GetCheckedInNames(), ","); got != "Grace" {
		t.Errorf("Ada sees %q checked in, want Grace", got)
	}
	if got := strings.Join(katherine.GetOutings()[0].GetCheckedInNames(), ","); got != "Ada,Grace" || katherine.GetOutings()[0].GetCheckedIn() {
		t.Errorf("Katherine sees %q checked in, herself checked in %v; want Ada,Grace and no", got, katherine.GetOutings()[0].GetCheckedIn())
	}
	if !mine.GetCheckInOpensAt().AsTime().Equal(outingStart.Add(-30*time.Minute)) || !mine.GetCheckInClosesAt().AsTime().Equal(outingEnd) ||
		!mine.GetFeedbackClosesAt().AsTime().Equal(outingEnd.Add(7*24*time.Hour)) {
		t.Errorf("windows: %v", mine)
	}
}

// Once checked in there is no cancel; a person who isn't going can't check in;
// someone with no seat finds no outing.
func TestCheckedInSeatsCannotCancelOverTheAPI(t *testing.T) {
	r := newOutingRig(t)
	_, _, _, outing := r.goingGroup(t)
	r.named(t, fourth, "fourth@example.com", "Mary Jackson")
	adaTok, _, _ := r.at(t, outingStart)
	if _, err := r.checkIn(adaTok, outing); err != nil {
		t.Fatal(err)
	}

	_, cancel := r.cancel(adaTok, outing)
	_, stranger := r.checkIn(r.signedIn(t, fourth, "fourth@example.com"), outing)
	_, missing := r.checkIn(adaTok, "not-an-outing")

	wantCode(t, cancel, connect.CodeFailedPrecondition)
	wantCode(t, stranger, connect.CodeNotFound)
	wantCode(t, missing, connect.CodeNotFound)
}

// Feedback over the API: closed before the plan's end, then the form lists the
// others by first name, answers are kept and changed, a "no" keeps nothing
// after it, a made-up handle is refused, and a person who wasn't going at the
// start is refused.
func TestFeedbackOverTheAPI(t *testing.T) {
	r := newOutingRig(t)
	_, _, _, outing := r.goingGroup(t)
	adaTok, _, _ := r.at(t, outingEnd.Add(-time.Second))
	_, early := r.feedback(adaTok, outing)
	adaTok, _, _ = r.at(t, outingEnd)

	form, err := r.feedback(adaTok, outing)
	if err != nil {
		t.Fatal(err)
	}
	var graceHandle, thirdHandle string
	for _, c := range form.GetOthers() {
		switch c.GetFirstName() {
		case "Grace":
			graceHandle = c.GetHandle()
		case "Katherine":
			thirdHandle = c.GetHandle()
		}
	}
	saved, err := r.saveFeedback(adaTok, &isshov1.SaveFeedbackRequest{OutingId: outing, Went: true, NotThere: []string{thirdHandle}, MeetAgain: []string{graceHandle}})
	if err != nil {
		t.Fatal(err)
	}
	_, bogus := r.saveFeedback(adaTok, &isshov1.SaveFeedbackRequest{OutingId: outing, Went: true, MeetAgain: []string{"nobody"}})
	no, err := r.saveFeedback(adaTok, &isshov1.SaveFeedbackRequest{OutingId: outing, Went: false, MeetAgain: []string{graceHandle}})
	if err != nil {
		t.Fatal(err)
	}
	late := r.clk.Now().Add(7 * 24 * time.Hour)
	r.clk.Set(late)
	adaTok = r.signedIn(t, ada, "ada@example.com")
	_, closed := r.feedback(adaTok, outing)

	wantCode(t, early, connect.CodeFailedPrecondition)
	wantCode(t, bogus, connect.CodeInvalidArgument)
	wantCode(t, closed, connect.CodeFailedPrecondition)
	if len(form.GetOthers()) != 2 || form.GetAnswered() || graceHandle == "" || thirdHandle == "" {
		t.Errorf("form = %v, want the two others, unanswered", form)
	}
	if !saved.GetAnswered() || !saved.GetWent() || len(saved.GetNotThere()) != 1 || saved.GetNotThere()[0] != thirdHandle || saved.GetMeetAgain()[0] != graceHandle {
		t.Errorf("saved = %v", saved)
	}
	if !no.GetAnswered() || no.GetWent() || len(no.GetNotThere()) != 0 || len(no.GetMeetAgain()) != 0 {
		t.Errorf("a no = %v, want nothing after it", no)
	}
}

// A person who only had a yes waiting, or never answered, is refused the form.
func TestFeedbackIsRefusedToSomeoneNotGoingAtTheStartOverTheAPI(t *testing.T) {
	r := newOutingRig(t)
	adaTok, graceTok, _ := r.formedGroup(t)
	outing := r.outingOf(t, adaTok).GetId()
	for _, tok := range []string{adaTok, graceTok} {
		r.answer(tok, outing, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)
	}
	// Katherine never answers, so the outing isn't set.
	adaTok, _, thirdTok := r.at(t, outingEnd)

	_, waiting := r.feedback(adaTok, outing)
	_, never := r.feedback(thirdTok, outing)

	wantCode(t, waiting, connect.CodeFailedPrecondition)
	wantCode(t, never, connect.CodeFailedPrecondition)
}

// "Who would you meet again?" is private: after one person picks another, what
// every call returns to the person picked is exactly what it returned before,
// and a mutual pick is told to neither.
func TestNoRPCTellsAPersonWhoPickedThem(t *testing.T) {
	r := newOutingRig(t)
	_, _, _, outing := r.goingGroup(t)
	adaTok, graceTok, _ := r.at(t, outingEnd)
	form, err := r.feedback(adaTok, outing)
	if err != nil {
		t.Fatal(err)
	}
	var graceHandle string
	for _, c := range form.GetOthers() {
		if c.GetFirstName() == "Grace" {
			graceHandle = c.GetHandle()
		}
	}
	// What Grace is sent, as bytes, by every call that returns her something.
	read := func(tok string) string {
		list, err := r.list(tok)
		if err != nil {
			t.Fatal(err)
		}
		f, err := r.feedback(tok, outing)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := protojson.Marshal(list)
		b, _ := protojson.Marshal(f)
		return string(a) + string(b)
	}
	before := read(graceTok)

	if _, err := r.saveFeedback(adaTok, &isshov1.SaveFeedbackRequest{OutingId: outing, Went: true, MeetAgain: []string{graceHandle}}); err != nil {
		t.Fatal(err)
	}

	if after := read(graceTok); after != before {
		t.Errorf("Grace is sent something different once Ada picks her:\nbefore %s\nafter  %s", before, after)
	}
}

// A yes at a time the caller is already going to another outing is refused over
// the API with the time-taken message, which the app's refusal copy reads.
func TestAYesAtATimeAlreadyTakenIsRefusedOverTheAPI(t *testing.T) {
	r := newOutingRig(t)
	ctx := context.Background()
	// The sunrise plan also runs in the evening, so two outings can share one.
	if _, err := r.pool.Exec(ctx, `update app.plans set parts_of_day = '{morning,evening}' where id = $1`, sunrise); err != nil {
		t.Fatal(err)
	}
	adaTok, graceTok, thirdTok := r.formedGroup(t)
	lanternOuting := r.outingOf(t, adaTok).GetId()
	fourthTok := r.named(t, fourth, "fourth@example.com", "Mary Jackson")
	fifthTok := r.named(t, "55555555-5555-4555-8555-555555555555", "fifth@example.com", "Dorothy Vaughan")
	for _, tok := range []string{adaTok, fourthTok, fifthTok} {
		if _, err := r.signUp(tok, sunrise, tick("2026-01-09", "evening")); err != nil {
			t.Fatal(err)
		}
	}
	if formed, err := r.forming.Form(ctx, sunrise); err != nil || formed != 1 {
		t.Fatalf("Form = %d, %v; want 1 outing", formed, err)
	}
	var sunriseOuting string
	list, err := r.list(adaTok)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range list.GetOutings() {
		if o.GetId() != lanternOuting {
			sunriseOuting = o.GetId()
		}
	}
	if sunriseOuting == "" {
		t.Fatalf("ada has no second outing: %v", list)
	}
	for _, tok := range []string{adaTok, graceTok, thirdTok} {
		if _, err := r.answer(tok, lanternOuting, isshov1.InvitationAnswer_INVITATION_ANSWER_YES); err != nil {
			t.Fatal(err)
		}
	}

	_, err = r.answer(adaTok, sunriseOuting, isshov1.InvitationAnswer_INVITATION_ANSWER_YES)

	wantCode(t, err, connect.CodeFailedPrecondition)
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Message() != "you are going to another outing at that time" {
		t.Fatalf("refusal = %v, want the time-taken message", err)
	}
}
