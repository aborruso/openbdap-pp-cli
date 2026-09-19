# Idee per il futuro

## Piu' codici in una sola invocazione

`cig A B C` (o `--file elenco.txt`), e lo stesso per `cup`. Oggi ogni codice paga la sua scansione dei dataset regionali; con piu' codici insieme la scansione si paga una volta. Proposto nei suggerimenti del 2026-09-19, rimandato perche' e' una funzione nuova, non una correzione.

## Indicatori MOP

Il form web «RICERCA PER CUP» del portale produce un Excel con il foglio «Dettaglio Indicatori», che nel catalogo open data non esiste. E' l'unico pezzo di MOP per cui serve il browser. Se si apre un canale verso RGS, chiedere la pubblicazione di quel dataset.

## Ingresso dal comune, e codici ISTAT

`opere --comune "Reggio Calabria"`, o un `--comune` su `dossier` e `cup`: oggi si entra solo da CUP, CIG o codice fiscale, e «cosa c'e' nel mio comune» non ha una porta.

Misurato il 2026-09-19 sul dump della localizzazione (667.780 righe) contro l'elenco dei comuni vigenti di SITUAS (`opensituas get 61`, 7.894 comuni):

- MOP ha 8.160 codici comune distinti e 7.994 denominazioni distinte;
- tutti i 7.894 comuni vigenti sono gia' in MOP, l'insieme «solo ISTAT» e' vuoto;
- 534 codici MOP non sono comuni vigenti (soppressi o fusi: Pecco, Camo, Rima San Giuseppe, Valmala), e su quei codici cadono 29.424 righe, il 4,41% del totale.

Quindi l'indice nome -> codice conviene costruirlo dal dataset di localizzazione durante `allinea`, non da ISTAT: e' completo per definizione e le stringhe combaciano con quelle che MOP contiene davvero, che non sono ne' quelle ufficiali ne' quelle d'uso comune. MOP scrive `BOLZANO` e non `BOLZANO/BOZEN`, `FORLI'` e non `FORLÌ`, `REGGIO DI CALABRIA` mentre chi digita scrive `REGGIO CALABRIA`, che oggi restituisce zero. Il matching tollerante serve comunque.

Da ISTAT serve una cosa sola: ricondurre i comuni estinti al successore, cioe' trovare anche le righe di Pecco quando si chiede Valchiusa. Sono i report SITUAS 99 (traslazione data inizio - data fine) e 128 (soppressi e non ricostituiti). Il 4,41% di righe e' oltre la soglia dell'assicurazione.

Vincoli se si fa: niente chiamata a `opensituas` a runtime, che e' Python mentre questa e' un binario Go che deve funzionare offline, quindi file generato una volta e committato; e file embeddato con `go:embed` invece di una tabella nuova nell'archivio, perche' lo schema dello store e' a senso unico e non vale un bump per una tabella di lookup statica.
