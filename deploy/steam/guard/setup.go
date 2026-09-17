package guard

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SetupResult is a newly linked authenticator.
type SetupResult struct {
	Account         *Account
	RevocationCode  string
	PhoneNumberHint string
	Path            string
}

// SetupAuthenticator logs in, AddAuthenticator, finalize with SMS, writes maFile.
func SetupAuthenticator(p Prompter, username, password string) (*SetupResult, error) {
	tok, err := LoginWithPassword(p, username, password)
	if err != nil {
		return nil, err
	}
	device := DeviceID(fmt.Sprintf("%d", tok.SteamID))
	if device == "android:" {
		device = randomDeviceID()
	}
	c := newSteamClient()
	add := concat(
		encodeFixed64(1, tok.SteamID),
		encodeVarintField(4, 1),
		encodeString(5, device),
		encodeString(6, "1"),
		encodeVarintField(8, 2),
	)
	am, _, err := c.call("ITwoFactorService", "AddAuthenticator", 1, add, tok.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("AddAuthenticator: %w (account must already have a phone on Steam)", err)
	}
	shared := am.bytes(1)
	ident := am.bytes(8)
	secret1 := am.bytes(9)
	rev := am.str(3)
	if len(shared) == 0 || rev == "" {
		return nil, fmt.Errorf("AddAuthenticator missing shared_secret or revocation_code")
	}
	if err := p.ConfirmRevocation(rev); err != nil {
		return nil, err
	}
	acc := &Account{
		AccountName:    firstNonEmpty(am.str(6), tok.AccountName),
		SteamID:        json.Number(fmt.Sprintf("%d", tok.SteamID)),
		SharedSecret:   base64.StdEncoding.EncodeToString(shared),
		IdentitySecret: base64.StdEncoding.EncodeToString(ident),
		Secret1:        base64.StdEncoding.EncodeToString(secret1),
		SerialNumber:   fmt.Sprintf("%d", am.u(2)),
		RevocationCode: rev,
		URI:            am.str(4),
		TokenGID:       am.str(7),
		DeviceID:       device,
	}
	sms, err := p.SMSCode()
	if err != nil {
		return nil, err
	}
	for i := 0; i < 8; i++ {
		now := time.Now().Unix()
		code, err := GenerateAuthCodeAt(acc.SharedSecret, now)
		if err != nil {
			return nil, err
		}
		fin := concat(
			encodeFixed64(1, tok.SteamID),
			encodeString(2, code),
			encodeVarintField(3, uint64(now)),
			encodeString(4, strings.TrimSpace(sms)),
			encodeBool(6, true),
		)
		fm, _, err := c.call("ITwoFactorService", "FinalizeAddAuthenticator", 1, fin, tok.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("FinalizeAddAuthenticator: %w", err)
		}
		if fm.b(2) { // want_more
			time.Sleep(2 * time.Second)
			continue
		}
		path, err := Save(acc)
		if err != nil {
			return nil, err
		}
		return &SetupResult{Account: acc, RevocationCode: rev, PhoneNumberHint: am.str(11), Path: path}, nil
	}
	return nil, fmt.Errorf("FinalizeAddAuthenticator still want_more")
}

// QueryStatus calls ITwoFactorService/QueryStatus.
func QueryStatus(accessToken string, steamID uint64) (map[string]any, error) {
	c := newSteamClient()
	m, _, err := c.call("ITwoFactorService", "QueryStatus", 1, encodeFixed64(1, steamID), accessToken)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"steamid":            steamID,
		"state":              m.u(1),
		"authenticator_type": m.u(3),
	}, nil
}

// RemoveAuthenticator calls ITwoFactorService/RemoveAuthenticator.
func RemoveAuthenticator(accessToken, revocation string) error {
	c := newSteamClient()
	body := concat(encodeString(2, revocation), encodeVarintField(6, 1))
	_, _, err := c.call("ITwoFactorService", "RemoveAuthenticator", 1, body, accessToken)
	return err
}

func randomDeviceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "android:" + hex.EncodeToString(b[:])
}
