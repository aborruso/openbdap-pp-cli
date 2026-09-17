// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// datasetProgettiTotale restituisce il dataset nazionale dei progetti, che e'
// l'unico che copre tutte le regioni; in mancanza, i progetti regionali.
func datasetProgetti(elenco []dataset, regione string) []dataset {
	if regione != "" {
		return datasetMOP(elenco, "progetti", regione)
	}
	if totale := datasetMOP(elenco, "progetti", "Totale"); len(totale) > 0 {
		return totale
	}
	return datasetMOP(elenco, "progetti", "")
}

func newNovelCupCmd(flags *rootFlags) *cobra.Command {
	// Il comando si costruisce qui con Use letterale: il controllo delle
	// funzioni originali cerca il nome nel sorgente, non a runtime.
	cmd := &cobra.Command{Use: "cup [codice]"}
	return configuraRicercaMOP(cmd, flags, ricercaMOP{
		nome:    "cup",
		breve:   "Cerca un progetto di opera pubblica dal suo codice CUP",
		lungo:   "Cerca il CUP nei dataset dei progetti, a partire dal dataset nazionale.\nUsa questo comando per l'anagrafica del progetto. NON usarlo per pagamenti, gare e partecipanti; usa 'dossier'.",
		colonna: "Codice CUP",
		esempio: strings.Trim(`
  openbdap-pp-cli cup I77H11000120009
  openbdap-pp-cli cup I77H11000120009 --agent
`, "\n"),
		happy:     "cup=I77H11000120009",
		famiglie:  []string{"progetti"},
		etichetta: "cup",
	})
}

func newNovelCigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "cig [codice]"}
	return configuraRicercaMOP(cmd, flags, ricercaMOP{
		nome:    "cig",
		breve:   "Cerca una gara e i suoi partecipanti dal codice CIG",
		lungo:   "Cerca il CIG nei dataset delle gare e dei partecipanti, che sono pubblicati per regione.\nUsa questo comando per una gara dal CIG. NON usarlo per cercare un progetto dal CUP; usa 'cup' o 'dossier'.",
		colonna: "Codice CIG",
		esempio: strings.Trim(`
  openbdap-pp-cli cig 12345678AB --regione Sicilia
  openbdap-pp-cli cig 12345678AB --agent
`, "\n"),
		happy:     "cig=12345678AB",
		famiglie:  []string{"gare", "partecipanti"},
		etichetta: "cig",
	})
}

// ricercaMOP descrive una ricerca per codice nei dataset MOP.
type ricercaMOP struct {
	nome      string
	breve     string
	lungo     string
	colonna   string
	esempio   string
	happy     string
	famiglie  []string
	etichetta string
}

// configuraRicercaMOP completa un comando di ricerca per codice nei dataset MOP.
func configuraRicercaMOP(cmd *cobra.Command, flags *rootFlags, r ricercaMOP) *cobra.Command {
	var regione, dbPath string
	var limite int

	cmd.Short = r.breve
	cmd.Long = r.lungo
	cmd.Example = r.esempio
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": r.happy}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, r.nome)
		}
		if len(args) == 0 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("indica il codice %s da cercare", strings.ToUpper(r.etichetta)))
		}
		codice := strings.ToUpper(strings.TrimSpace(args[0]))
		elenco, ok, err := datasetLocali(cmd, flags, dbPath)
		if err != nil {
			return err
		}
		if !ok {
			// L'avviso sullo standard error non basta: chi legge solo lo
			// standard output vedrebbe "nessun risultato" come un fatto.
			return printJSONFiltered(cmd.OutOrStdout(), rispostaLocale{
				Richiesta: codice,
				Risultati: make([]esitoFamiglia, 0),
				Nota:      notaArchivioVuoto,
			}, flags)
		}
		var bersagli []dataset
		for _, fam := range r.famiglie {
			if fam == "progetti" {
				bersagli = append(bersagli, datasetProgetti(elenco, regione)...)
				continue
			}
			bersagli = append(bersagli, datasetMOP(elenco, fam, regione)...)
		}
		if len(bersagli) == 0 {
			return fmt.Errorf("nessun dataset MOP nell'archivio locale: lancia 'openbdap-pp-cli allinea'")
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		esiti := cercaNeiMOP(cmd.Context(), c, bersagli, r.colonna, codice, limite)

		trovate := 0
		falliti := 0
		risultati := make([]esitoFamiglia, 0, len(esiti))
		for _, e := range esiti {
			if e.Errore != "" {
				falliti++
				risultati = append(risultati, e)
				continue
			}
			if len(e.Righe) == 0 {
				continue
			}
			trovate += len(e.Righe)
			risultati = append(risultati, e)
		}
		if falliti > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "attenzione: %d dataset su %d non hanno risposto; il risultato e' parziale\n", falliti, len(esiti))
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printJSONFiltered(cmd.OutOrStdout(), risultati, flags)
		}
		if trovate == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Nessun risultato per %s %s nei dataset interrogati (%d).\n", strings.ToUpper(r.etichetta), codice, len(esiti))
			return nil
		}
		tabella := make([]map[string]any, 0, trovate)
		for _, e := range risultati {
			for _, riga := range e.Righe {
				voce := map[string]any{"famiglia": e.Famiglia, "regione": e.Regione}
				for k, v := range riga {
					voce[k] = v
				}
				tabella = append(tabella, voce)
			}
		}
		return printAutoTable(cmd.OutOrStdout(), tabella)
	}
	cmd.Flags().StringVar(&regione, "regione", "", "limita la ricerca a una regione")
	cmd.Flags().IntVar(&limite, "limite", 50, "numero massimo di righe per dataset")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

func newNovelOpereCmd(flags *rootFlags) *cobra.Command {
	var cf, regione, dbPath string
	var limite int
	var soloConteggio bool

	cmd := &cobra.Command{
		Use:   "opere",
		Short: "Le opere pubbliche in capo a un ente, dal suo codice fiscale",
		Long: "Elenca le opere il cui soggetto titolare ha il codice fiscale indicato, con il totale reale restituito dal servizio.\n" +
			"Usa questo comando per le opere di un ente. NON usarlo per un singolo progetto di cui conosci il CUP; usa 'cup' o 'dossier'.",
		Example: strings.Trim(`
  openbdap-pp-cli opere --cf 80208450587 --solo-conteggio
  openbdap-pp-cli opere --cf 80208450587 --limite 5 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--cf=80208450587;--solo-conteggio"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "opere")
			}
			if strings.TrimSpace(cf) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--cf e' obbligatorio: indica il codice fiscale del soggetto titolare"))
			}
			elenco, ok, err := datasetLocali(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"codice_fiscale": strings.TrimSpace(cf),
					"opere":          make([]map[string]any, 0),
					"totale":         nil,
					"nota":           notaArchivioVuoto,
				}, flags)
			}
			bersagli := datasetProgetti(elenco, regione)
			if len(bersagli) == 0 {
				return fmt.Errorf("nessun dataset dei progetti nell'archivio locale: lancia 'openbdap-pp-cli allinea'")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			totale := 0
			var falliti int
			righe := make([]map[string]any, 0)
			for _, d := range bersagli {
				colonne, err := colonneDataset(ctx, c, d.ODataID)
				if err != nil {
					falliti++
					continue
				}
				col, ok := risolviColonna(colonne, "Codice Fiscale Titolare")
				if !ok {
					falliti++
					continue
				}
				filtro := fmt.Sprintf("%s eq '%s'", col.ID, strings.ReplaceAll(strings.TrimSpace(cf), "'", "''"))
				n, err := contaRighe(ctx, c, d.ODataID, filtro)
				if err != nil {
					falliti++
					continue
				}
				totale += n
				if soloConteggio || n == 0 {
					continue
				}
				blocco, err := righeDataset(ctx, c, d.ODataID, colonne, filtro, nil, limite, 0)
				if err != nil {
					falliti++
					continue
				}
				for _, riga := range blocco {
					voce := map[string]any{"regione": d.Regione}
					for k, v := range riga {
						voce[k] = v
					}
					righe = append(righe, voce)
				}
			}
			if falliti > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "attenzione: %d dataset su %d non hanno risposto; il totale e' calcolato sui restanti\n", falliti, len(bersagli))
			}
			esito := map[string]any{
				"codice_fiscale":          strings.TrimSpace(cf),
				"totale":                  totale,
				"dataset_interrogati":     len(bersagli),
				"dataset_non_rispondenti": falliti,
				"opere":                   righe,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), esito, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d opere per il codice fiscale %s\n", totale, strings.TrimSpace(cf))
			if len(righe) == 0 {
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), righe)
		},
	}
	cmd.Flags().StringVar(&cf, "cf", "", "codice fiscale del soggetto titolare")
	cmd.Flags().StringVar(&regione, "regione", "", "limita la ricerca a una regione")
	cmd.Flags().IntVar(&limite, "limite", 20, "numero massimo di opere da elencare per dataset")
	cmd.Flags().BoolVar(&soloConteggio, "solo-conteggio", false, "restituisci solo il numero di opere")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelCupCmd(flags))
	})
}
