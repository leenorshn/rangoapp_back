# Caisse et paiements en espèces

Le contrat est `POST /graphql`. Les montants sont des chaînes décimales (`"20"`, `"50000"`, `"0"`). Les dates sont des `DateTime` RFC3339. USD et CDF ne sont jamais additionnés : chaque montant porte sa devise.

L'ouverture, l'encaissement et la clôture s'exécutent chacun dans une transaction. Une caisse n'a qu'une session `open`. Une session `closed` n'accepte plus ni vente, ni dépense, ni second comptage. Rejouer `confirmSale` avec la même `idempotencyKey` et le même contenu renvoie la même vente, sans second paiement ni second mouvement.

## Ouvrir la caisse

`openCashSession` enregistre le vendeur connecté, la boutique, la caisse et les fonds initiaux. Un montant nul est accepté. Un montant négatif est refusé. La caisse doit appartenir à la boutique.

```graphql
mutation OpenCash($input: OpenCashSessionInput!) {
  openCashSession(input: $input) {
    id
    storeId
    cashRegisterId
    openedBy
    openedAt
    status
    openingBalances { currency amount }
    theoreticalBalances { currency amount }
  }
}
```

```json
{
  "input": {
    "storeId": "STORE_ID",
    "cashRegisterId": "REGISTER_ID",
    "openingBalances": [
      { "currency": "USD", "amount": "20" },
      { "currency": "CDF", "amount": "50000" }
    ]
  }
}
```

Réponse utile : `id` devient `cashSessionId` pour les ventes. `status` vaut `open`. `theoreticalBalances` reprend les fonds initiaux tant qu'aucun mouvement n'a eu lieu.

`cashRegisterId` vient de `stores { cashRegister { id name } }`.

## Lire la caisse

`cash(storeId)` liste les caisses de la boutique et la session ouverte, s'il y en a une.

`cashSession(id)` et `cashSessions(storeId, status, after, afterId, limit)` détaillent une session ou l'historique. `status` vaut `open`, `closed`, ou est omis. Les sessions sont renvoyées de la plus récente à la plus ancienne. Le curseur est le couple (`openedAt`, `id`) du dernier élément reçu : les deux champs partent ensemble.

`cashTransactions(sessionId, after, afterId, limit)` lit le journal du plus ancien au plus récent. Le curseur est (`createdAt`, `id`).

```graphql
query Drawer($storeId: ID!, $sessionId: ID!) {
  cash(storeId: $storeId) {
    id
    name
    openSession {
      id
      status
      openedBy
      openingBalances { currency amount }
      totalIn { currency amount }
      totalOut { currency amount }
      theoreticalBalances { currency amount }
    }
  }
  cashSession(id: $sessionId) {
    status
    closedAt
    countedBalances { currency amount }
    variances { currency amount }
    note
    transactions { id kind direction currency amount createdAt }
  }
}
```

`balances` et `theoreticalBalances` sont le même solde théorique par devise :

`solde théorique = fonds initial + entrées - sorties`

`totalIn` ne contient pas le fonds initial. `totalOut` contient les sorties, par exemple une dépense payée depuis la caisse. `variances` et `countedBalances` sont vides tant que la session est ouverte.

Si l'appelant n'est ni celui qui a ouvert la session, ni un rôle qui lit les rapports, les listes de montants et le journal sont `null`. L'identité de la session reste visible.

## Encaisser une vente

`confirmSale` reçoit `cashSessionId` dès qu'un paiement a `method: "cash"`. Le champ reste absent pour une vente sans espèces. Un paiement mixte n'écrit en caisse que la part `cash`. La devise du mouvement est celle de la vente.

```graphql
mutation Sell($input: ConfirmSaleInput!) {
  confirmSale(input: $input) {
    saleId
    currency
    netAmount
    paidAmount
    dueAmount
    debtId
  }
}
```

```json
{
  "input": {
    "idempotencyKey": "ticket-2026-10-16-0042",
    "storeId": "STORE_ID",
    "currency": "USD",
    "cashSessionId": "SESSION_ID",
    "lines": [
      {
        "productId": "PRODUCT_ID",
        "unitName": "piece",
        "quantity": "8",
        "expectedUnitPrice": "10"
      }
    ],
    "payments": [
      { "method": "cash", "amount": "80" }
    ]
  }
}
```

Une vente CDF est le même appel avec `currency: "CDF"` et des montants CDF. La session peut recevoir les deux devises dans la journée. Android conserve une clé d'idempotence par ticket et renvoie exactement le même corps en cas de timeout.

## Clôturer

Le vendeur compte l'argent réellement présent. L'écart est `montant compté - solde théorique`, par devise, sans conversion. Un écart négatif ou positif est conservé. Aucun mouvement correctif n'est créé.

```graphql
mutation CloseCash($input: CloseCashSessionInput!) {
  closeCashSession(input: $input) {
    id
    status
    closedBy
    closedAt
    theoreticalBalances { currency amount }
    countedBalances { currency amount }
    variances { currency amount }
    note
  }
}
```

```json
{
  "input": {
    "cashSessionId": "SESSION_ID",
    "note": "comptage du soir",
    "countedBalances": [
      { "currency": "USD", "amount": "88" },
      { "currency": "CDF", "amount": "205000" }
    ]
  }
}
```

Exemple : ouverture 20 USD et 50 000 CDF, ventes 80 USD et 150 000 CDF, sortie 10 USD. Le solde théorique est 90 USD et 200 000 CDF. Un comptage de 88 USD et 205 000 CDF donne un écart de -2 USD et +5 000 CDF.

Chaque devise qui a un mouvement, y compris un fonds initial à zéro, doit être comptée. `note` est facultative. `reports.cashCounts` reprend ces écarts sur les sessions clôturées de la période.

## Erreurs à afficher

Le code stable est dans `errors[].extensions.code`. Le message est en français.

| Code | Situation | Côté Android |
| --- | --- | --- |
| `cash_session_open` | Cette caisse a déjà une session ouverte | Reprendre la session renvoyée par `cash` |
| `cash_session` | Espèces sans session, session fermée, autre boutique, ou caisse d'un autre caissier | Ouvrir la caisse, ou encaisser et clôturer la session ouverte par cet utilisateur |
| `cash_session_closed` | Seconde clôture | Afficher la session déjà clôturée |
| `opening` | Fonds initial négatif, devise invalide, ou devise répétée | Corriger la saisie. `0` est valide. La devise est en majuscules, `USD` ou `CDF` |
| `count` | Devise manquante, répétée, ou montant compté négatif | Demander chaque devise mouvementée, une seule fois |
| `not_found` | Caisse, boutique ou session invisible pour cette entreprise | L'identifiant appartient à une autre société ou n'existe pas |
| `cash.open`, `cash.close`, `store` | Rôle, affectation ou abonnement insuffisant | Le vendeur (`seller`) et le caissier ouvrent et clôturent leur propre session. Ils ne clôturent pas celle d'un collègue : ce cas renvoie `cash_session`. Un propriétaire ou un gérant le peut |

Une vente confirmée après un timeout se reconnaît au même `saleId`. Si la clôture et une vente se croisent, une seule des deux écritures l'emporte : soit la vente est dans le solde théorique clôturé, soit elle est refusée et absente du journal.
