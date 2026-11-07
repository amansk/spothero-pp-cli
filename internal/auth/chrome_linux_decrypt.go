package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// importLinuxStdlib reads the Chrome cookie DB via Python's stdlib sqlite3
// (no browser_cookie3, no sqlite3 CLI) and decrypts v10 values using the
// Linux "basic" password-store key ("peanuts"), as used by headless Chrome.
func importLinuxStdlib(cookiesDB string) (*Session, error) {
	script := `
import sys, json, sqlite3, shutil, tempfile, os
d = tempfile.mkdtemp(); p = os.path.join(d, "c.db"); shutil.copy(sys.argv[1], p)
c = sqlite3.connect(p)
v = int((c.execute("select value from meta where key='version'").fetchone() or ["0"])[0])
rows = [[n, val, ev.hex()] for n, val, ev in c.execute("select name, value, encrypted_value from cookies where host_key like '%spothero%' order by length(host_key)")]
shutil.rmtree(d)
print(json.dumps({"version": v, "rows": rows}))
`
	out, err := exec.Command("python3", "-c", script, cookiesDB).Output()
	if err != nil {
		return nil, fmt.Errorf("read chrome cookies via python3 sqlite3: %w", err)
	}
	var res struct {
		Version int         `json:"version"`
		Rows    [][3]string `json:"rows"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, err
	}
	key := pbkdf2SHA1([]byte("peanuts"), []byte("saltysalt"), 1, 16)
	cookies := map[string]string{}
	for _, r := range res.Rows {
		if r[1] != "" {
			cookies[r[0]] = r[1]
			continue
		}
		enc, _ := hex.DecodeString(r[2])
		val, err := decryptChromeV10(enc, key, res.Version >= 24)
		if err != nil {
			continue
		}
		cookies[r[0]] = val
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no decryptable spothero.com cookies in chrome; sign in at https://spothero.com first")
	}
	return &Session{Cookies: cookies, Source: "chrome", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func decryptChromeV10(enc, key []byte, stripDomainHash bool) (string, error) {
	if len(enc) < 3 || (string(enc[:3]) != "v10" && string(enc[:3]) != "v11") {
		return "", fmt.Errorf("unsupported cookie encryption")
	}
	data := enc[3:]
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return "", fmt.Errorf("bad ciphertext length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := []byte("                ")
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, data)
	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(plain) {
		return "", fmt.Errorf("bad padding (wrong key?)")
	}
	plain = plain[:len(plain)-pad]
	if stripDomainHash {
		if len(plain) < 32 {
			return "", fmt.Errorf("short plaintext")
		}
		plain = plain[32:]
	}
	return string(plain), nil
}

// pbkdf2SHA1 is a minimal PBKDF2-HMAC-SHA1 (avoids an x/crypto dependency).
func pbkdf2SHA1(password, salt []byte, iter, keyLen int) []byte {
	var out []byte
	for i := 1; len(out) < keyLen; i++ {
		mac := hmac.New(sha1.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for n := 1; n < iter; n++ {
			mac = hmac.New(sha1.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
