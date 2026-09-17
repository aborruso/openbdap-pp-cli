// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"openbdap-pp-cli/internal/cliutil"
	"openbdap-pp-cli/internal/store"
)

// notaArchivioVuoto e' la spiegazione che accompagna ogni risposta vuota
// dovuta a un archivio locale non ancora popolato. Viaggia nell'output, non
// solo sullo standard error, perche' un agente legge solo il primo.
const notaArchivioVuoto = "archivio locale assente o vuoto: lancia 'openbdap-pp-cli allinea' prima di cercare"

// rispostaLocale e' l'involucro delle risposte che leggono l'archivio locale.
// Anche quando non trova nulla resta informativa: riporta la richiesta e dice
// quale comando popola l'archivio, invece di restituire una lista vuota muta.
type rispostaLocale struct {
	Richiesta string `json:"richiesta,omitempty"`
	Risultati any    `json:"risultati"`
	Trovati   int    `json:"trovati"`
	Nota      string `json:"nota,omitempty"`
}

// apriStore apre l'archivio locale del catalogo. Restituisce ok=false quando
// l'archivio non esiste ancora: chi chiama stampa un risultato vuoto.
func apriStore(cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, bool, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "nessun archivio locale in %s\nlancia: openbdap-pp-cli allinea\n", dbPath)
		return nil, false, nil
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, false, err
	}
	return db, true, nil
}

// datasetLocali apre l'archivio e restituisce tutti i dataset allineati.
func datasetLocali(cmd *cobra.Command, flags *rootFlags, dbPath string) ([]dataset, bool, error) {
	db, ok, err := apriStore(cmd, flags, dbPath)
	if err != nil || !ok {
		return nil, ok, err
	}
	defer db.Close()
	elenco, err := leggiDataset(db)
	if err != nil {
		return nil, false, err
	}
	return elenco, true, nil
}

// risolviLocale risolve un dataset dall'archivio e verifica che abbia una
// risorsa OData: senza quella non ci sono righe da leggere.
func risolviLocale(cmd *cobra.Command, flags *rootFlags, dbPath, chiave string, servonoRighe bool) (dataset, bool, error) {
	elenco, ok, err := datasetLocali(cmd, flags, dbPath)
	if err != nil || !ok {
		return dataset{}, ok, err
	}
	d, trovato := trovaDataset(elenco, chiave)
	if !trovato {
		return dataset{}, false, usageErr(fmt.Errorf("dataset %q non trovato nell'archivio locale: cercalo con 'openbdap-pp-cli cerca %s'", chiave, chiave))
	}
	if servonoRighe && d.ODataID == "" {
		return dataset{}, false, fmt.Errorf("il dataset %q non pubblica una risorsa OData: scaricalo in CSV con 'openbdap-pp-cli scarica %s'", d.Titolo, d.ID)
	}
	return d, true, nil
}

func newAllineaCmd(flags *rootFlags) *cobra.Command {
	var paralleli int
	var tema string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "allinea",
		Short: "Allinea il catalogo dei dataset nell'archivio locale",
		Long: "Scarica i metadati di tutti i dataset del catalogo e li conserva in un archivio SQLite locale, " +
			"con indice full-text su titolo e descrizione.\n" +
			"Usa questo comando prima di 'cerca', 'serie' e 'mop'. NON usarlo per leggere le righe di un dataset; usa 'righe'.",
		Example: strings.Trim(`
  openbdap-pp-cli allinea
  openbdap-pp-cli allinea --tema 172_opere-pubbliche
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "allinea")
			}
			// Il client generato applica gia' --timeout a ogni richiesta:
			// legare l'intero comando a quella durata troncherebbe gli
			// allineamenti lunghi e le estrazioni impaginate.
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var ids []string
			if tema != "" {
				ids, err = datasetDelGruppo(ctx, c, tema)
			} else {
				ids, err = elencoDataset(ctx, c)
			}
			if err != nil {
				return err
			}
			if cliutil.IsDogfoodEnv() && len(ids) > 20 {
				ids = ids[:20]
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			contati, errori := scaricaDataset(ctx, c, ids, paralleli, func(d dataset) error {
				return salvaDataset(db, d)
			})
			esito := map[string]any{
				"dataset_allineati": contati,
				"dataset_richiesti": len(ids),
				"archivio":          dbPath,
			}
			if len(errori) > 0 {
				falliti := make([]string, 0, len(errori))
				for _, e := range errori {
					falliti = append(falliti, e.Error())
				}
				if len(falliti) > 10 {
					falliti = falliti[:10]
				}
				esito["errori"] = len(errori)
				esito["dettaglio_errori"] = falliti
				fmt.Fprintf(cmd.ErrOrStderr(), "attenzione: %d dataset su %d non sono stati allineati\n", len(errori), len(ids))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), esito, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Allineati %d dataset su %d in %s\n", contati, len(ids), dbPath)
			return nil
		},
	}
	cmd.Flags().IntVar(&paralleli, "paralleli", 6, "richieste in parallelo verso il portale")
	cmd.Flags().StringVar(&tema, "tema", "", "allinea solo i dataset di un tema, per esempio 172_opere-pubbliche")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

func newCercaCmd(flags *rootFlags) *cobra.Command {
	var limite int
	var tema, tag, anno, regione, famiglia string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "cerca [testo]",
		Short: "Cerca dataset nell'archivio locale, per titolo e descrizione",
		Long: "Cerca nel catalogo allineato in locale, con ricerca full-text su titolo e descrizione e filtri per tema, parola chiave, anno e regione.\n" +
			"Usa questo comando per trovare un dataset. NON usarlo per elencare le annualita' di una serie; usa 'serie'.",
		Example: strings.Trim(`
  openbdap-pp-cli cerca "opere pubbliche"
  openbdap-pp-cli cerca SIOPE --anno 2024 --regione Sicilia
  openbdap-pp-cli cerca --tema 172_opere-pubbliche --limite 5
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "testo=opere"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cerca")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("'cerca' legge solo l'archivio locale: allinea il catalogo con 'openbdap-pp-cli allinea'"))
			}
			testoCercato := strings.Join(args, " ")
			elenco, ok, err := datasetLocali(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			trovati := make([]dataset, 0, limite)
			if ok {
				for _, d := range elenco {
					if !corrisponde(d, testoCercato, tema, tag, anno, regione, famiglia) {
						continue
					}
					trovati = append(trovati, d)
					if limite > 0 && len(trovati) >= limite {
						break
					}
				}
			}
			risposta := rispostaLocale{Richiesta: testoCercato, Risultati: trovati, Trovati: len(trovati)}
			if len(trovati) == 0 {
				risposta.Nota = "nessun dataset corrisponde: se l'archivio locale e' vuoto lancia 'openbdap-pp-cli allinea'"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), risposta, flags)
			}
			if len(trovati) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nessun dataset corrisponde ai criteri. Se l'archivio e' vuoto, lancia 'openbdap-pp-cli allinea'.")
				return nil
			}
			righe := make([]map[string]any, 0, len(trovati))
			for _, d := range trovati {
				righe = append(righe, map[string]any{
					"titolo": d.Titolo, "anno": d.Anno, "regione": d.Regione, "id": d.ID, "odata": d.ODataID,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), righe)
		},
	}
	cmd.Flags().IntVar(&limite, "limite", 20, "numero massimo di dataset da restituire (0 = tutti)")
	cmd.Flags().StringVar(&tema, "tema", "", "filtra per tema, per esempio 172_opere-pubbliche")
	cmd.Flags().StringVar(&tag, "tag", "", "filtra per parola chiave")
	cmd.Flags().StringVar(&anno, "anno", "", "filtra per anno di riferimento ricavato dal titolo")
	cmd.Flags().StringVar(&regione, "regione", "", "filtra per regione ricavata dal titolo")
	cmd.Flags().StringVar(&famiglia, "famiglia", "", "filtra per famiglia MOP: progetti, gare, partecipanti, pagamenti, piano-costi, soggetti-titolari")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

// corrisponde applica i filtri della ricerca a un dataset.
func corrisponde(d dataset, testoCercato, tema, tag, anno, regione, famiglia string) bool {
	if testoCercato != "" {
		ago := normalizza(d.Titolo) + " " + normalizza(d.Note) + " " + normalizza(d.Nome)
		for _, parola := range strings.Fields(normalizza(testoCercato)) {
			if !strings.Contains(ago, parola) {
				return false
			}
		}
	}
	if tema != "" && !strings.Contains(normalizza(d.Tema), normalizza(tema)) {
		return false
	}
	if tag != "" && !strings.Contains(normalizza(d.Tag), normalizza(tag)) {
		return false
	}
	if anno != "" && d.Anno != anno {
		return false
	}
	if regione != "" && normalizza(d.Regione) != normalizza(regione) {
		return false
	}
	if famiglia != "" && normalizza(d.Famiglia) != normalizza(famiglia) {
		return false
	}
	return true
}

func newColonneCmd(flags *rootFlags) *cobra.Command {
	var conValori bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "colonne [dataset]",
		Short: "Mostra le colonne di un dataset, con il nome da usare nei filtri",
		Long: "Elenca i campi di un dataset con nome leggibile, nome fisico, identificativo da usare nei filtri, tipo e cardinalita'.\n" +
			"Usa questo comando per i campi di un dataset che gia' conosci. NON usarlo per scoprire in quali dataset vive un campo; usa 'campi'.",
		Example: strings.Trim(`
  openbdap-pp-cli colonne bda1676b-62ab-44b7-8f9a-ca93b8534488
  openbdap-pp-cli colonne "Progetti Opere Pubbliche MOP - Totale" --valori
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "dataset=bda1676b-62ab-44b7-8f9a-ca93b8534488"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "colonne")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indica il dataset da ispezionare"))
			}
			// Il client generato applica gia' --timeout a ogni richiesta:
			// legare l'intero comando a quella durata troncherebbe gli
			// allineamenti lunghi e le estrazioni impaginate.
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			odataID := args[0]
			if d, ok, err := risolviLocaleMorbido(cmd, flags, dbPath, args[0]); err != nil {
				return err
			} else if ok {
				odataID = d.ODataID
			}
			colonne, err := colonneDataset(ctx, c, odataID)
			if err != nil {
				return err
			}
			if !conValori {
				for i := range colonne {
					colonne[i].Valori = nil
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), colonne, flags)
			}
			righe := make([]map[string]any, 0, len(colonne))
			for _, col := range colonne {
				righe = append(righe, map[string]any{
					"nome": col.Nome, "id_filtro": col.ID, "tipo": col.Tipo, "cardinalita": col.Cardinalita,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), righe)
		},
	}
	cmd.Flags().BoolVar(&conValori, "valori", false, "includi i valori distinti pubblicati per ogni colonna")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

// risolviLocaleMorbido prova a risolvere un dataset dall'archivio; se manca
// l'archivio o la corrispondenza, chi chiama usa il valore cosi' com'e'.
func risolviLocaleMorbido(cmd *cobra.Command, flags *rootFlags, dbPath, chiave string) (dataset, bool, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return dataset{}, false, nil
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return dataset{}, false, err
	}
	defer db.Close()
	elenco, err := leggiDataset(db)
	if err != nil {
		return dataset{}, false, err
	}
	d, ok := trovaDataset(elenco, chiave)
	if !ok || d.ODataID == "" {
		return dataset{}, false, nil
	}
	return d, true, nil
}

func newRigheCmd(flags *rootFlags) *cobra.Command {
	var dove []string
	var campi []string
	var limite, salta int
	var tutte bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "righe [dataset]",
		Short: "Estrae le righe di un dataset, filtrando per nome di colonna leggibile",
		Long: "Legge le righe di un dataset via OData. I filtri accettano il nome leggibile della colonna, " +
			"che viene tradotto nell'identificativo richiesto dal servizio.\n" +
			"Usa campo=valore per l'uguaglianza e campo~testo per la ricerca parziale.",
		Example: strings.Trim(`
  openbdap-pp-cli righe bda1676b-62ab-44b7-8f9a-ca93b8534488 --limite 10
  openbdap-pp-cli righe bda1676b-62ab-44b7-8f9a-ca93b8534488 --dove "Codice CUP=I77H11000120009"
  openbdap-pp-cli righe bda1676b-62ab-44b7-8f9a-ca93b8534488 --dove "Descrizione Titolare~COMUNE DI PALERMO" --campi "Codice CUP,Descrizione CUP Integrale" --csv
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "dataset=bda1676b-62ab-44b7-8f9a-ca93b8534488;--limite=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "righe")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indica il dataset da leggere"))
			}
			// Il client generato applica gia' --timeout a ogni richiesta:
			// legare l'intero comando a quella durata troncherebbe gli
			// allineamenti lunghi e le estrazioni impaginate.
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			odataID := args[0]
			if d, ok, err := risolviLocaleMorbido(cmd, flags, dbPath, args[0]); err != nil {
				return err
			} else if ok {
				odataID = d.ODataID
			}
			colonne, err := colonneDataset(ctx, c, odataID)
			if err != nil {
				return err
			}
			filtro, err := costruisciFiltro(colonne, dove)
			if err != nil {
				return usageErr(err)
			}
			pagina := limite
			if pagina <= 0 || pagina > 1000 {
				pagina = 1000
			}
			if cliutil.IsDogfoodEnv() {
				tutte = false
				if pagina > 50 {
					pagina = 50
				}
			}
			var righe []map[string]any
			offset := salta
			for {
				blocco, err := righeDataset(ctx, c, odataID, colonne, filtro, campi, pagina, offset)
				if err != nil {
					return err
				}
				righe = append(righe, blocco...)
				if !tutte || len(blocco) < pagina {
					break
				}
				if limite > 0 && len(righe) >= limite {
					break
				}
				offset += pagina
			}
			if limite > 0 && len(righe) > limite {
				righe = righe[:limite]
			}
			if righe == nil {
				righe = make([]map[string]any, 0)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), righe, flags)
			}
			if len(righe) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nessuna riga corrisponde al filtro.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), righe)
		},
	}
	cmd.Flags().StringArrayVar(&dove, "dove", nil, "condizione campo=valore oppure campo~testo, ripetibile")
	cmd.Flags().StringSliceVar(&campi, "campi", nil, "colonne da restituire, separate da virgola")
	cmd.Flags().IntVar(&limite, "limite", 50, "numero massimo di righe (0 = nessun limite con --tutte)")
	cmd.Flags().IntVar(&salta, "salta", 0, "righe da saltare")
	cmd.Flags().BoolVar(&tutte, "tutte", false, "impagina fino a esaurire le righe che soddisfano il filtro")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

func newContaCmd(flags *rootFlags) *cobra.Command {
	var dove []string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "conta [dataset]",
		Short: "Conta le righe di un dataset, anche con un filtro",
		Long: "Restituisce il numero di righe di un dataset. E' l'unico conteggio affidabile del servizio: " +
			"il conteggio standard di OData restituisce sempre zero.",
		Example: strings.Trim(`
  openbdap-pp-cli conta bda1676b-62ab-44b7-8f9a-ca93b8534488
  openbdap-pp-cli conta bda1676b-62ab-44b7-8f9a-ca93b8534488 --dove "Codice Fiscale Titolare=80208450587"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "dataset=bda1676b-62ab-44b7-8f9a-ca93b8534488"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "conta")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indica il dataset da contare"))
			}
			// Il client generato applica gia' --timeout a ogni richiesta:
			// legare l'intero comando a quella durata troncherebbe gli
			// allineamenti lunghi e le estrazioni impaginate.
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			odataID := args[0]
			titolo := ""
			if d, ok, err := risolviLocaleMorbido(cmd, flags, dbPath, args[0]); err != nil {
				return err
			} else if ok {
				odataID, titolo = d.ODataID, d.Titolo
			}
			filtro := ""
			if len(dove) > 0 {
				colonne, err := colonneDataset(ctx, c, odataID)
				if err != nil {
					return err
				}
				filtro, err = costruisciFiltro(colonne, dove)
				if err != nil {
					return usageErr(err)
				}
			}
			totale, err := contaRighe(ctx, c, odataID, filtro)
			if err != nil {
				return err
			}
			esito := map[string]any{"dataset": odataID, "righe": totale}
			if titolo != "" {
				esito["titolo"] = titolo
			}
			if filtro != "" {
				esito["filtro"] = filtro
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), esito, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d righe\n", totale)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&dove, "dove", nil, "condizione campo=valore oppure campo~testo, ripetibile")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newAllineaCmd(flags))
		addNovelCommandIfAbsent(root, newCercaCmd(flags))
		addNovelCommandIfAbsent(root, newColonneCmd(flags))
		addNovelCommandIfAbsent(root, newRigheCmd(flags))
		addNovelCommandIfAbsent(root, newContaCmd(flags))
	})
}
