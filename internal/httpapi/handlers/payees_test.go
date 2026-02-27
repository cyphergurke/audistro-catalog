package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPostPayeeForExistingArtistReturns201(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	artistRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Handle:      "alice",
		DisplayName: "Alice",
	})
	if artistRec.Code != http.StatusCreated {
		t.Fatalf("create artist status=%d body=%s", artistRec.Code, artistRec.Body.String())
	}

	payeeRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/payees", CreatePayeeRequest{
		ArtistHandle:     "alice",
		FAPPublicBaseURL: "https://fap.artist.example",
		FAPPayeeID:       "payee_abc",
	})
	if payeeRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", payeeRec.Code, payeeRec.Body.String())
	}
}

func TestPostPayeeUnknownArtistReturns404(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	payeeRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/payees", CreatePayeeRequest{
		ArtistHandle:     "unknown",
		FAPPublicBaseURL: "https://fap.artist.example",
		FAPPayeeID:       "payee_abc",
	})
	if payeeRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", payeeRec.Code, payeeRec.Body.String())
	}
}

func TestGetPayeeWorks(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	_ = doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Handle:      "alice",
		DisplayName: "Alice",
	})
	createPayeeRec := doJSONRequest(t, app.handler, http.MethodPost, "/v1/payees", CreatePayeeRequest{
		ArtistHandle:     "alice",
		FAPPublicBaseURL: "https://fap.artist.example",
		FAPPayeeID:       "payee_abc",
	})
	if createPayeeRec.Code != http.StatusCreated {
		t.Fatalf("create payee status=%d body=%s", createPayeeRec.Code, createPayeeRec.Body.String())
	}

	var createResp PayeeResponse
	if err := json.NewDecoder(createPayeeRec.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create payee response: %v", err)
	}

	getPayeeRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/payees/"+createResp.Payee.PayeeID, nil)
	if getPayeeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getPayeeRec.Code, getPayeeRec.Body.String())
	}

	var getResp PayeeResponse
	if err := json.NewDecoder(getPayeeRec.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode get payee response: %v", err)
	}
	if getResp.Payee.PayeeID != createResp.Payee.PayeeID {
		t.Fatalf("expected payee_id %q got %q", createResp.Payee.PayeeID, getResp.Payee.PayeeID)
	}
}

func TestGetArtistPayeesListReturnsCreatedPayees(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)

	_ = doJSONRequest(t, app.handler, http.MethodPost, "/v1/artists", CreateArtistRequest{
		PubKeyHex:   "fedcbafedcbafedcbafedcbafedcbafedcbafedcbafedcbafedcbafedcbafedc",
		Handle:      "alice",
		DisplayName: "Alice",
	})

	_ = doJSONRequest(t, app.handler, http.MethodPost, "/v1/payees", CreatePayeeRequest{
		ArtistHandle:     "alice",
		FAPPublicBaseURL: "https://fap1.artist.example",
		FAPPayeeID:       "payee_one",
	})
	_ = doJSONRequest(t, app.handler, http.MethodPost, "/v1/payees", CreatePayeeRequest{
		ArtistHandle:     "alice",
		FAPPublicBaseURL: "https://fap2.artist.example",
		FAPPayeeID:       "payee_two",
	})

	listRec := doJSONRequest(t, app.handler, http.MethodGet, "/v1/artists/alice/payees", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp PayeesListResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list payees response: %v", err)
	}
	if len(listResp.Payees) != 2 {
		t.Fatalf("expected 2 payees, got %d", len(listResp.Payees))
	}
}
