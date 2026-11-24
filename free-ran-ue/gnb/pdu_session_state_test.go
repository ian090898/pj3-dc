package gnb

import (
	"bytes"
	"testing"
)

func TestRanUePerSessionTeids(t *testing.T) {
	r := NewRanUe(nil, NewRanUeNgapIdGenerator())

	dl4 := []byte{0, 0, 0, 1}
	ul4 := []byte{0, 0, 0, 2}
	dl5 := []byte{0, 0, 0, 3}
	ul5 := []byte{0, 0, 0, 4}

	// Legacy methods must continue operating on session 4.
	r.SetDlTeid(dl4)
	r.SetUlTeid(ul4)

	r.SetPduSessionDlTeid(5, dl5)
	r.SetPduSessionUlTeid(5, ul5)

	if !bytes.Equal(r.GetDlTeid(), dl4) ||
		!bytes.Equal(r.GetUlTeid(), ul4) {
		t.Fatal("legacy session 4 TEIDs changed")
	}

	if !bytes.Equal(r.GetPduSessionDlTeid(5), dl5) ||
		!bytes.Equal(r.GetPduSessionUlTeid(5), ul5) {
		t.Fatal("session 5 TEIDs are incorrect")
	}

	// Modifying session 5 must not overwrite session 4.
	r.SetPduSessionUlTeid(5, []byte{0, 0, 0, 9})
	if !bytes.Equal(r.GetUlTeid(), ul4) {
		t.Fatal("session 5 overwrote session 4")
	}

	// The table must not share mutable TEID byte slices.
	dl4[3] = 99
	if !bytes.Equal(r.GetDlTeid(), []byte{0, 0, 0, 1}) {
		t.Fatal("stored TEID was modified through the input")
	}

	got := r.GetPduSessionDlTeid(5)
	got[3] = 99
	if !bytes.Equal(r.GetPduSessionDlTeid(5), dl5) {
		t.Fatal("stored TEID was modified through the getter")
	}

	if r.GetPduSessionDlTeid(6) != nil {
		t.Fatal("unexpected session 6")
	}
}

func TestRanUeReleaseAllSessionTeids(t *testing.T) {
	idGenerator := NewRanUeNgapIdGenerator()
	teidGenerator := NewTeidGenerator()
	r := NewRanUe(nil, idGenerator)

	dl4 := teidGenerator.AllocateTeid()
	dl5 := teidGenerator.AllocateTeid()

	r.SetDlTeid(dl4)
	r.SetPduSessionDlTeid(5, dl5)

	r.Release(idGenerator, teidGenerator)

	count := 0
	teidGenerator.teids.Range(func(_, _ any) bool {
		count++
		return true
	})

	if count != 0 {
		t.Fatalf("expected all TEIDs released, got %d", count)
	}
}

func TestRanUeSessionCompletionCarriesID(t *testing.T) {
	r := NewRanUe(nil, NewRanUeNgapIdGenerator())

	go func() {
		r.GetPduSessionEstablishmentCompleteChan() <- 5
	}()

	got := <-r.GetPduSessionEstablishmentCompleteChan()
	if got != 5 {
		t.Fatalf("got completed session %d, want 5", got)
	}
}
