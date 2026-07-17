// Package web embeds the ClassKhata single-page UI so the server ships as
// one self-contained binary.
package web

import "embed"

//go:embed index.html style.css app.js
var Files embed.FS
