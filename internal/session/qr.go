package session

import (
	"encoding/base64"

	qrcode "github.com/skip2/go-qrcode"
)

// qrDataURI renders text as a QR code PNG and returns it as a data: URI
// ready to drop straight into an <img src>. Returns "" if generation fails
// (the connection string is still shown as selectable text either way).
func qrDataURI(text string) string {
	png, err := qrcode.Encode(text, qrcode.Medium, 512)
	if err != nil {
		return ""
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}
