package presentation

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed art/skiff.svg
var mark []byte

var icons = map[string]*ui.SVG{"boat": ui.MustParseSVG(mark)}

func init() {
	// Angular silhouettes and clipped corners keep these legible at toolbar size.
	symbols := map[string]string{
		"theme":    "M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18z M12 3v18 M15 6l-3 3m5-1-5 5m6-2-6 6m5-1-4 4",
		"light":    "M2 14c4 1 6-2 10-6s7-4 10-2c-4-1-6 3-10 7s-7 5-10 1z M4 18c6 2 11-8 16-8",
		"settings": "M3 7h8m4 0h6M3 17h3m4 0h11 M13 4l3 3-3 3-3-3z M8 14l3 3-3 3-3-3z",
		"clock":    "M8 3h8l5 5v8l-5 5H8l-5-5V8z M12 6v6l4 2",
		"file":     "M6 3h8l4 5v13H6z M14 3v6h4",
		"restart":  "M4 5v14 M8 12l11-7v14z",
		"all":      "M3 5h7v6H3z M14 5h7v4l-2 2h-5z M3 15h7v5H3z M14 15h7v5h-7z",
		"network":  "M9 3h6v5H9z M3 16h6v5H3z M15 16h6v5h-6z M12 8v4M6 16v-4h12v4",
		"download": "M9 3h6v7h4l-7 7-7-7h4z M3 17v3h15l3-3",
		"pause":    "M5 7l4-3h1v15H5z M15 5h4v12l-4 3z",
		"play":     "M6 4l14 8-14 8z M6 4l4 8-4 8",
		"check":    "M2.5 12l4-3 4 4 9-9 2 3-11 13z",
		"error":    "M8 3h8l5 5v8l-5 5H8l-5-5V8z M12 7v6 M12 16v1",
		"folder":   "M3 9V6h7l3 3h8v2 M3 11h18l-3 8H3z",
		"close":    "M5 5h2l12 14 M19 5h-2L5 19",
		"retry":    "M20 10l-4-5H8l-5 6 M20 4v6h-6 M4 14l4 5h8l5-6 M4 20v-6h6",
		"plus":     "M11 3h2v8h8v2h-8v8h-2v-8H3v-2h8z",
		"search":   "M7 3h6l4 4v6l-4 4H7l-4-4V7z M16 16l5 5",
		"tray":     "M3 13l4-8h10l4 8v5l-3 2H3z M3 13h5l2 3h4l2-3h5",
		"chevron":  "M6 9l6 6 6-6",
	}
	for name, d := range symbols {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round"><path d="` + d + `"/></svg>`))
	}
}

// The broad faces read as metal even at the smaller size used in file lists.
var fileArt = map[string]*ui.SVG{
	"archive": metalObject(`<path d="m7 18 25-12 25 12-25 14z" fill="url(#light)"/><path d="m7 18 25 14v26L7 44z" fill="url(#silver)"/><path d="m32 32 25-14v26L32 58z" fill="url(#shadow)"/><path d="m18 12 24 14 6-3L24 9z" fill="#77879e"/><path d="m42 26 6-3v9l-6 4z" fill="#b1c3cf"/><path d="m12 37 12 7v3l-12-7z" fill="#f8f9fa"/>`),
	"audio":   metalObject(`<path d="m5 28 25-18 29 12v17L34 56 5 44z" fill="url(#shadow)"/><path d="m5 28 25-18 29 12-25 19z" fill="url(#light)"/><path d="m5 28 29 13v15L5 44z" fill="url(#silver)"/><path d="m17 28 7-5 12 5 11-8 3 1-14 11-12-5-4 3z" fill="#556479"/><path d="m11 37 12 5v3l-12-5z" fill="#e4eef5"/>`),
	"media":   metalObject(`<path d="m9 20 34-11 14 8v30L23 57 9 49z" fill="url(#shadow)"/><path d="m9 20 34-11 14 8-34 11z" fill="url(#light)"/><path d="m9 20 14 8v29L9 49z" fill="url(#silver)"/><path d="m32 31 15 3-15 12z" fill="url(#light)"/>`),
	"file":    metalObject(`<path d="m13 12 27-7 12 10v36l-31 8-8-5z" fill="url(#shadow)"/><path d="m13 12 27-7v13l9-2v33l-36 9z" fill="url(#silver)"/><path d="M40 5 52 15l-12 3z" fill="url(#light)"/><path d="m21 30 20-5v3l-20 5z m0 8 14-4v3l-14 4z" fill="#65758d"/>`),
}

// A folded transfer slip suspended over an open receiving tray.
var inboxArt = metalObject(`
    <path d="m4 40 32-13 25 9-30 18z" fill="url(#silver)"/>
    <path d="m10 40 26-10 19 7-24 12z" fill="url(#shadow)"/>
    <path d="m4 40 18 6 4 6 9-5 2-5 24-8v10L30 61 4 51z" fill="url(#light)"/>
    <path d="m30 55 5-3 4-7 22-9v8L30 61z" fill="url(#silver)"/>
    <path d="m13 22 34-17-8 27-12 4-5-8z" fill="url(#light)"/>
    <path d="M22 28 47 5 27 30l1 6z" fill="url(#shadow)"/>
    <path d="M27 30 47 5l-8 27z" fill="url(#silver)"/>`)

func metalObject(faces string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><defs>
    <linearGradient id="light" x2="95%" y2="80%"><stop stop-color="#fff"/><stop offset=".53" stop-color="#e2ecf4"/><stop offset="1" stop-color="#adbad0"/></linearGradient>
    <linearGradient id="silver" x2="100%" y2="90%"><stop stop-color="#d2d7ea"/><stop offset=".43" stop-color="#95a7bd"/><stop offset=".78" stop-color="#d2dbe2"/><stop offset="1" stop-color="#a3a9bd"/></linearGradient>
    <linearGradient id="shadow" x2="60%" y2="100%"><stop stop-color="#8997ac"/><stop offset=".63" stop-color="#526178"/><stop offset="1" stop-color="#778197"/></linearGradient>
    </defs>` + faces + `</svg>`))
}
