# Vente rapide EasyStock pour Android

Contrat `POST /graphql`. Les montants et les quantités sont des chaînes décimales (`"21500"`, `"1.5"`, `"24"`), jamais des flottants. `createdAt` est un `DateTime` RFC3339. `debtDueDate` et `dueOn` sont des dates civiles `AAAA-MM-JJ`, sans heure, pour éviter un décalage de fuseau. L'entreprise vient de la session.

Le serveur recalcule les prix, les sous-totaux et le stock. Le total affiché par Android n'est pas enregistré. `confirmSale` est la seule mutation de validation.

## Droits

Le vendeur et le caissier peuvent vendre, ouvrir leur caisse, créer un client et enregistrer une dette. Un plafond de crédit s'applique seulement s'il est configuré sur l'entreprise (`default_max_credit`) ou par une délégation. Un refus explicite de `credit.authorize` bloque la dette et laisse la vente comptant possible. Le magasinier ne vend pas. Un abonnement expiré refuse la vente (`SUBSCRIPTION_EXPIRED`).

`openDebts` est renseigné pour un rôle qui peut lire les dettes (propriétaire, responsable, caissier). Il est `null` pour le vendeur.

## 1. Charger les produits

```graphql
query ($storeId: ID!, $search: String, $after: DateTime, $afterId: ID) {
  products(storeId: $storeId, search: $search, forSale: true, after: $after, afterId: $afterId, limit: 50) {
    id
    name
    sku
    barcode
    imageMediaId
    baseUnit
    availableQuantity
    availability
    units {
      id
      name
      factor
      barcode
      allowSale
      allowFractional
      prices { currency unitPrice }
    }
  }
}
```

```json
{ "storeId": "<storeId>", "search": "eau" }
```

`search` porte sur le nom, le SKU et le code-barres du produit ou d'un conditionnement. `forSale: true` retire les produits archivés et ceux dont aucun conditionnement actif n'est vendable. La page suivante reprend `updatedAt` et `id` du dernier élément.

`availableQuantity` est le stock de la boutique dans l'unité de base. `availability` vaut `available` ou `out_of_stock`. Cette valeur sert à l'affichage. La confirmation relit le stock et le prix.

Un scan exact utilise `productByBarcode(storeId, barcode)`. `matchedUnitId` indique le conditionnement reconnu.

S'il y a un `imageMediaId`, l'URL se charge à part :

```graphql
query ($productId: ID!) {
  productMedia(productId: $productId) { downloadUrl contentType }
}
```

## 2. Unités et prix

Chaque conditionnement a un `factor` : `1 carton = 24` unités de base. Le prix à envoyer est `prices[].unitPrice` pour la devise de la vente et ce conditionnement. Un conditionnement sans prix dans cette devise n'est pas vendable (`not_found`).

Exemple affiché :

- 2 cartons à `10000` CDF
- 3 bouteilles à `500` CDF

Android peut montrer `21500` CDF. Le serveur refera le calcul.

## 3. Panier

Une ligne contient :

- `productId`
- `unitName` : le nom du conditionnement, pas son identifiant
- `quantity` : quantité de ce conditionnement, supérieure à zéro
- `expectedUnitPrice` : le `unitPrice` affiché

`"1.5"` n'est accepté que si `allowFractional` est vrai sur ce conditionnement (`quantity` sinon). Vendre 2 cartons et 3 bouteilles déduit `51` bouteilles. Deux lignes du même produit sont autorisées.

## 4. Client

Recherche par nom, téléphone ou WhatsApp. Un numéro local `097…` est comparé à sa forme `+243…`.

```graphql
query ($storeId: ID!, $search: String!) {
  customers(storeId: $storeId, search: $search, limit: 20) {
    id
    name
    phone
    whatsappPhone
    address
    openDebts { currency amount }
  }
}
```

```json
{ "storeId": "<storeId>", "search": "Amina" }
```

Création sans quitter la vente. Le nom, l'adresse et au moins un numéro sont obligatoires. Le téléphone et le WhatsApp sont normalisés en E.164 (RDC, indicatif `+243`). Le WhatsApp est facultatif quand le téléphone principal sert déjà. Un même numéro ne peut pas être repris par un autre client de l'entreprise (`already_exists`). Une autre entreprise peut utiliser le même numéro.

```graphql
mutation ($input: CreateCustomerInput!) {
  createCustomer(input: $input) {
    id
    name
    phone
    whatsappPhone
    address
  }
}
```

```json
{
  "input": {
    "storeId": "<storeId>",
    "name": "Amina Kabila",
    "address": "Avenue de la Paix 12",
    "phone": "0970000001",
    "whatsappPhone": "0970000001"
  }
}
```

Réponse : `"phone": "+243970000001"`. Conserver `id` pour `confirmSale`.

## 5. Vente cash

Une session de caisse ouverte dans cette boutique est obligatoire dès qu'un paiement `cash` est présent. Le paiement `external` est un encaissement déclaré (mobile money, par exemple) : il ne demande pas de session et n'entre pas dans le tiroir.

Générer une `idempotencyKey` UUID avant l'appel et la réutiliser telle quelle en cas de timeout. Ne pas en générer une nouvelle pour le même panier.

```graphql
mutation ($input: ConfirmSaleInput!) {
  confirmSale(input: $input) {
    saleId
    reference
    currency
    netAmount
    paidAmount
    dueAmount
    createdAt
    itemCount
  }
}
```

```json
{
  "input": {
    "idempotencyKey": "<uuid>",
    "storeId": "<storeId>",
    "currency": "CDF",
    "cashSessionId": "<cashSessionId>",
    "lines": [
      { "productId": "<eau>", "unitName": "carton", "quantity": "2", "expectedUnitPrice": "10000" },
      { "productId": "<eau>", "unitName": "bouteille", "quantity": "3", "expectedUnitPrice": "500" }
    ],
    "payments": [{ "method": "cash", "amount": "21500" }]
  }
}
```

`reference` est égal à `saleId`. L'écran de succès affiche la référence, `netAmount`, `paidAmount`, `currency`, `createdAt` et `itemCount` (nombre de lignes).

## 6. Vente avec dette

Le solde est `netAmount - somme des paiements`. Les paiements sont les lignes `payments`, pas un champ `advanceAmount` isolé.

- Aucun paiement, ou une liste vide : avance `0`. Pas de session de caisse.
- Somme inférieure au total : dette. `customerId` et `debtDueDate` sont obligatoires.
- Somme égale au total : vente soldée. Aucune dette n'est créée, même si une date est envoyée.
- Somme supérieure au total : refus (`payment`). Pas de monnaie rendue.
- Montant négatif ou nul sur une ligne : refus (`payment`).

Le stock est déduit même si l'avance est nulle. L'avance cash alimente la session. L'avance externe est enregistrée sans mouvement de tiroir. L'avance n'est pas un remboursement ultérieur : `collectedAmount` de la dette démarre à zéro.

```json
{
  "input": {
    "idempotencyKey": "<uuid>",
    "storeId": "<storeId>",
    "customerId": "<customerId>",
    "currency": "CDF",
    "cashSessionId": "<cashSessionId>",
    "debtDueDate": "2026-10-23",
    "lines": [
      { "productId": "<productId>", "unitName": "pièce", "quantity": "1", "expectedUnitPrice": "50000" }
    ],
    "payments": [{ "method": "cash", "amount": "15000" }]
  }
}
```

Pour une avance nulle, envoyer `"payments": []` et omettre `cashSessionId`.

`debtDueDate` est le jour prévu, `AAAA-MM-JJ`, dans le fuseau de la boutique. Le jour de la vente est accepté. Un jour précédent est refusé (`debt_due_date`).

## 7. Confirmation avec dette

Demander aussi :

```graphql
confirmSale(input: $input) {
  saleId
  reference
  customerId
  customerName
  currency
  netAmount
  paidAmount
  dueAmount
  debtId
  debtDueDate
  createdAt
  itemCount
}
```

Exemple de réponse pour un panier de `50000`, une avance de `15000` et une échéance au `2026-10-23` :

```json
{
  "data": {
    "confirmSale": {
      "saleId": "<saleId>",
      "reference": "<saleId>",
      "customerId": "<customerId>",
      "customerName": "Amina Kabila",
      "currency": "CDF",
      "netAmount": "50000",
      "paidAmount": "15000",
      "dueAmount": "35000",
      "debtId": "<debtId>",
      "debtDueDate": "2026-10-23",
      "createdAt": "2026-10-16T12:00:00Z",
      "itemCount": 1
    }
  }
}
```

Afficher « Vente effectuée avec succès » puis « Dette enregistrée : 35 000 CDF » quand `dueAmount` est supérieur à zéro. La vente est enregistrée même si elle n'est pas soldée.

Sur la dette relue plus tard :

- `originalAmount` est le solde initial, ici `35000`, pas le total du panier
- `advanceAmount` est le montant payé à la vente, ici `15000`
- `collectedAmount` augmente seulement lors d'un remboursement ultérieur
- `dueAmount` est le reste
- `dueOn` est l'échéance persistée
- `status` vaut `open` (impayée), `partial` ou `settled` (payée)
- `overdue` est vrai quand l'échéance est dépassée et qu'il reste un solde

`debts(storeId, status: "overdue")` filtre ces créances. Le statut stocké ne devient pas `overdue`.

## 8. Erreurs

Le code stable est `extensions.code`.

| Code | Quand | Quoi faire |
|---|---|---|
| `price_changed` | `expectedUnitPrice` ne correspond plus au prix serveur | Relire le produit, afficher le nouveau prix, renvoyer avec une nouvelle clé |
| `stock_conflict` | Stock insuffisant | Réduire la quantité ou retirer la ligne |
| `product_archived` | Produit archivé | Retirer la ligne |
| `unit_archived` | Conditionnement archivé | Choisir un autre conditionnement |
| `unit_not_for_sale` | Conditionnement non vendable | Choisir un conditionnement `allowSale: true` |
| `quantity` | Quantité nulle, négative ou fractionnaire interdite | Corriger la quantité |
| `not_found` | Produit, conditionnement, prix, client ou caisse introuvable | Recharger la donnée |
| `payment` | Montant négatif, nul, ou supérieur au total | Ajuster les paiements |
| `customer` | Solde restant sans client | Choisir ou créer le client |
| `debt_due_date` | Dette sans échéance, date illisible ou antérieure | Choisir une date du jour ou ultérieure |
| `cash_session` | Espèces sans session ouverte dans cette boutique | Ouvrir la caisse de la boutique |
| `credit_ceiling` | Dette au-dessus du plafond configuré | Réduire le crédit ou encaisser le reste |
| `credit.authorize` | Le rôle ne peut pas faire crédit | Encaisser en cash, ou demander une délégation |
| `sales.create` | Le rôle ne peut pas vendre | — |
| `already_exists` | Téléphone déjà utilisé dans l'entreprise | Sélectionner le client existant |
| `phone` | Numéro absent ou invalide | Corriger le numéro |
| `idempotency_mismatch` | Même clé, contenu différent | Garder la même clé et le même corps, ou créer une nouvelle clé pour une autre vente |
| `SUBSCRIPTION_EXPIRED` | Abonnement expiré | La vente est refusée |

Un même appel répété avec la même clé et le même corps renvoie la vente déjà créée. Il ne déduit pas le stock une seconde fois.

La vente, les lignes, les sorties de stock, les paiements, le mouvement de caisse et la dette sont enregistrés dans une seule transaction. Une erreur ne laisse pas de vente partielle.
