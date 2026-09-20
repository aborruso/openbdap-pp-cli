# Esempi per giornalisti

Comandi verificati il 2026-09-20 sulla CLI installata. Gli output riportati sono quelli veri: se cambiano, sono cambiati i dati del portale.

## Cosa c'e' dentro MOP, in breve

Il Monitoraggio Opere Pubbliche nasce dal d.lgs. 229/2011: chi realizza un'opera pubblica e' obbligato a trasmettere alla BDAP l'anagrafica, i soldi e lo stato di avanzamento, e il legame fra CUP (il progetto) e CIG (la gara) tiene insieme le due meta'. Nel catalogo open data quel patrimonio esce come sette famiglie di dataset, sei pubblicate per regione e una nazionale:

| famiglia | dataset | cosa contiene | righe in Sicilia |
|---|---|---|---|
| progetti | 22 | anagrafica del CUP, titolare, settore, tipologia, quadro economico previsto ed effettivo, finanziamenti per fonte, date di progettazione ed esecuzione | 27.733 |
| gare | 21 | CIG, oggetto, tipo di procedura, data, importo a base d'asta e di aggiudicazione, numero di partecipanti | 52.162 |
| partecipanti | 21 | chi ha partecipato a ogni gara, mandatario e mandanti con codice fiscale | 27.412 |
| pagamenti | 21 | importo pagato per CUP e per anno | 43.618 |
| piano-costi | 21 | importo da realizzare e realizzato per CUP e per anno | 45.786 |
| soggetti-titolari | 21 | titolare del CUP con forma giuridica e comune di sede | 27.733 |
| localizzazione | 1 | dove ricade l'opera: regione, provincia e comune ISTAT | nazionale |

Quello che in MOP **non** c'e', pur esistendo nel sistema a monte: gli indicatori di avanzamento fisico (A17 del manuale MOP), che il form web restituisce nell'Excel «Dettaglio Indicatori» e che nel catalogo open data non hanno un dataset corrispondente. Restano fuori anche i SAL per singolo stato di avanzamento, le varianti e le sospensioni.

Due avvertenze da tenere nel pezzo. Circa meta' dei CUP non sta in MOP, quindi un `trovati: 0` significa «non e' in questo archivio», non «non esiste». E un costo effettivo a zero significa che in MOP non risulta speso nulla: e' una domanda da fare all'ente, non una conclusione da scrivere.

## Cosa c'e' nel mio comune

```bash
openbdap-pp-cli conta c4cce647-cec4-4b60-a8ab-d308ecfba743 --dove "Descrizione Comune=PALERMO"
openbdap-pp-cli righe c4cce647-cec4-4b60-a8ab-d308ecfba743 --dove "Descrizione Comune=PALERMO" --campi "Codice CUP" --tutte --csv > cup-palermo.csv
```

1459 opere localizzate a Palermo, e la lista dei CUP pronta per il passo dopo. I nomi sono quelli del portale: `REGGIO DI CALABRIA` e `FORLI'`, non le forme d'uso comune, che restituiscono zero.

## La scheda di una singola opera

```bash
openbdap-pp-cli dossier D93G04000030048 --agent | jq '{titolare: .results.progetto[0]["Descrizione Titolare"], dove: .results.sezioni.localizzazione[0]["Descrizione Comune"], gare: (.results.sezioni.gare|length), pagamenti: (.results.sezioni.pagamenti|length)}'
```

Comune di Palermo, Palermo, zero gare, zero pagamenti: e' il riuso dell'ex Chimica Arenella, CUP aperto nel 2004 per 65 milioni previsti. Una chiamata sola al posto di cinque query costruite a mano, e lo zero e' affidabile, perche' un archivio locale mancante si dichiara con `archivio_vuoto: true` invece di passare per una risposta.

## Chi muove i soldi in una regione

```bash
openbdap-pp-cli scarica 9ffc0c78-36e6-45a9-8f7e-1082a35a8329 --output sicilia.csv

duckdb -c "select \"Descrizione Titolare\" ente, count(*) opere,
  round(sum(try_cast(\"Costo Lavori Previsto\" as double))/1e6,1) previsto_mln
from read_csv_auto('sicilia.csv', delim=';', header=true, all_varchar=true)
where \"Descrizione Stato CUP\"='ATTIVO' group by all order by previsto_mln desc limit 8"
```

In testa RFI con 196 opere e 24,4 miliardi, poi Stretto di Messina con una sola opera da 10,5 miliardi, ANAS con 1544 opere e 7,4 miliardi. Il CSV va dritto in DuckDB perche' `scarica` lo converte in UTF-8; con `--raw` tornano i byte latin-1 del portale e gli accenti si rompono.

## Le opere ferme da vent'anni

```bash
duckdb -c "select \"Codice CUP\" cup, left(\"Descrizione CUP Integrale\",60) oggetto,
  \"Data Inizio Validità CUP\" inizio, round(try_cast(\"Costo Lavori Previsto\" as double)/1e6,1) mln
from read_csv_auto('sicilia.csv', delim=';', header=true, all_varchar=true)
where \"Descrizione Stato CUP\"='ATTIVO' and try_cast(\"Costo Lavori Effettivo\" as double)=0
  and try_cast(\"Costo Lavori Previsto\" as double)>50000000
  and \"Data Inizio Validità CUP\"<'2012-01-01' order by mln desc limit 6"
```

Catania-Roccapalumba-Palermo, 540 milioni, CUP del 2004. Velocizzazione Gela-Siracusa, 190 milioni, 2006. Ospedale Civico, 86 milioni, 2007. Ogni riga e' un CUP da passare a `dossier` per vedere se almeno una gara c'e' stata.

## Chi vince le gare

```bash
openbdap-pp-cli cig 57106934F1 --regione Sicilia --agent | jq '.results.risultati[] | {famiglia, righe: (.righe|length)}'
```

Dal CIG si arriva alla gara e ai partecipanti, mandatario e mandanti con il codice fiscale. Senza `--regione` la ricerca attraversa tutte le partizioni e costa una ventina di secondi; con la regione sta in due.

## Il conteggio di un ente

```bash
openbdap-pp-cli opere --cf 80208450587 --solo-conteggio --agent | jq '{ente: .results.codice_fiscale, opere: .results.totale}'
```

13.489 opere ANAS. Sullo stesso filtro l'API del portale risponde `"__count": "0"`: e' il motivo per cui questa CLI esiste.
