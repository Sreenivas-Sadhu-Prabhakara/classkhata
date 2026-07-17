// ClassKhata server: JSON API + embedded web UI in one binary.
package main

import (
	"log"
	"net/http"
	"os"

	"classkhata/internal/api"
	"classkhata/internal/core"
	"classkhata/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8105"
	}

	store, err := core.NewStore("data/store.json")
	if err != nil {
		log.Fatalf("classkhata: %v", err)
	}

	// The mock provider is the only shipped implementation. Setting
	// WHATSAPP_PROVIDER=live is acknowledged but falls back to mock until a
	// live WhatsApp Business Cloud API client is added.
	var provider core.Provider = core.MockWhatsApp{}
	if os.Getenv("WHATSAPP_PROVIDER") == "live" {
		log.Println("classkhata: WHATSAPP_PROVIDER=live requested but no live client is shipped; using mock")
	}

	handler := api.New(store, provider, web.Files)
	log.Printf("ClassKhata listening on http://localhost:%s (anchor date %s, whatsapp=%s)", port, core.AnchorDate, provider.Mode())
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
