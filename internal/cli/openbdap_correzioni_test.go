// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestRegioneDallaLocalizzazione(t *testing.T) {
	righe := []map[string]any{
		{"Codice Regione": "19", "Descrizione Regione": "SICILIA"},
	}
	if got := regioneDallaLocalizzazione(righe); got != "Sicilia" {
		t.Fatalf("regione = %q, attesa Sicilia", got)
	}
	if got := regioneDallaLocalizzazione(nil); got != "" {
		t.Fatalf("senza righe la regione deve restare vuota, ottenuto %q", got)
	}
	if got := regioneDallaLocalizzazione([]map[string]any{{"Codice Regione": "99"}}); got != "" {
		t.Fatalf("un codice sconosciuto non deve inventare una regione, ottenuto %q", got)
	}
}

func TestAggiungiCodiceISTAT(t *testing.T) {
	riga := map[string]any{"Codice Provincia": "007", "Codice Comune": "058"}
	aggiungiCodiceISTAT(riga)
	if riga["Codice ISTAT Comune"] != "007058" {
		t.Fatalf("codice ISTAT = %v, atteso 007058", riga["Codice ISTAT Comune"])
	}
	parziale := map[string]any{"Codice Provincia": "007"}
	aggiungiCodiceISTAT(parziale)
	if _, presente := parziale["Codice ISTAT Comune"]; presente {
		t.Fatal("senza il codice comune non si deve scrivere un codice ISTAT monco")
	}
}

func TestFamiglieTroncate(t *testing.T) {
	esiti := []esitoFamiglia{
		{Famiglia: "gare", Righe: make([]map[string]any, 50)},
		{Famiglia: "gare", Righe: make([]map[string]any, 50)},
		{Famiglia: "partecipanti", Righe: make([]map[string]any, 3)},
	}
	got := famiglieTroncate(esiti, 50)
	if len(got) != 1 || got[0] != "gare" {
		t.Fatalf("famiglie troncate = %v, attesa solo gare una volta", got)
	}
	if famiglieTroncate(esiti, 0) != nil {
		t.Fatal("senza limite non c'e' troncamento")
	}
}

func TestLettoreUTF8ConvertLatin1(t *testing.T) {
	// "Citt\xe0" in latin-1.
	dati, err := io.ReadAll(nuovoLettoreUTF8(bytes.NewReader([]byte("Comune;Citt\xe0\n"))))
	if err != nil {
		t.Fatal(err)
	}
	if string(dati) != "Comune;Città\n" {
		t.Fatalf("convertito = %q", string(dati))
	}
}

func TestLettoreUTF8LasciaPassareUTF8(t *testing.T) {
	originale := "Comune;Città\nPalermo;sì\n"
	dati, err := io.ReadAll(nuovoLettoreUTF8(strings.NewReader(originale)))
	if err != nil {
		t.Fatal(err)
	}
	if string(dati) != originale {
		t.Fatalf("un flusso gia' UTF-8 non va toccato: %q", string(dati))
	}
}

func TestLettoreUTF8DestinazioneStretta(t *testing.T) {
	lettore := nuovoLettoreUTF8(bytes.NewReader([]byte("a\xe0b")))
	var fuori bytes.Buffer
	buf := make([]byte, 1)
	for {
		n, err := lettore.Read(buf)
		fuori.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if fuori.String() != "aàb" {
		t.Fatalf("letto a un byte per volta = %q", fuori.String())
	}
}

func TestDichiaraUTF8(t *testing.T) {
	if !dichiaraUTF8("text/csv; charset=UTF-8") {
		t.Fatal("un Content-Type che dichiara UTF-8 va riconosciuto")
	}
	if dichiaraUTF8("text/csv") {
		t.Fatal("senza charset non si deve dare per buono UTF-8")
	}
}
