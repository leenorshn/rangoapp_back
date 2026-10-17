# État d’implémentation EasyStock

Date : 12 octobre 2026. Le serveur Mongo a été retiré du dépôt. Le binaire est `cmd/easystock` (`migrate`, `grpc`, `api`, `import`).

## En place et couvert par des tests PostgreSQL 16

Postgres embarqué 16 (`embedded-postgres`), rôle `easystock_app` sans `SUPERUSER` ni `BYPASSRLS`.

- Vente à crédit atomique, rejeu d’idempotence, clé réutilisée avec un autre corps refusée.
- Deux ventes concurrentes de la dernière unité : une seule réussit.
- Deux confirmations concurrentes de la même clé : une seule vente.
- Échec après écriture : aucune vente, stock inchangé.
- Remboursement qui réintègre le stock revendable.
- Clôture de caisse immuable, correction sur une nouvelle session.
- Isolation d’entreprise, y compris après `SET row_security = off`, et contexte de session vidé en fin de transaction.
- Export de marge refusé au caissier ; autorisé au propriétaire sous le nom `gross_margin`, quelle que soit l'offre.
- Abonnement expiré : nouvelle vente refusée, consultation des ventes encore possible.
- L'offre ne limite plus les boutiques, les rôles ni les marges. Un propriétaire ne change pas son propre rôle. L'expiration de l'abonnement bloque l'accès.
- Paiement local `source=local`, refus de chevauchement avec une période Clerk, webhook dupliqué et événement périmé ignoré.
- gRPC : sans jeton, `Unauthenticated` ; entreprise forgée, `PermissionDenied` ; le second appel avec la même clé renvoie la même vente.
- Import JSON : répétition à blanc annulée, second import sans nouvelle vente, UUID stables.
- Curseur catalogue, y compris produit archivé.
- Jeton Clerk RS256 (émetteur, expiration, partie autorisée) et chargement JWKS.
- Création d'entreprise avec essai de 14 jours, première boutique, invitation et acceptation. Une entreprise non liée se sélectionne par `X-EasyStock-Company`. Deux entreprises sans organisation Clerk ne sont pas fusionnées au hasard.
- GraphQL lit les ventes, clients, dettes, stock, caisse et rapports. Le coût n'est pas dans le stock. La marge brute suit le rôle, pas l'offre. Un caissier ne voit que ses ventes. Une boutique d'une autre entreprise est introuvable.
- GraphQL `confirmSale` envoie des chaînes décimales et le jeton de contexte interne. `POST /graphql` refuse un bearer qui n'est pas une session Clerk.
- Taux de change par entreprise. `convertAmount` utilise le sens enregistré, sinon l'inverse, et n'additionne jamais deux devises. Un caissier convertit ; seul le propriétaire ou l'administrateur enregistre le taux. Une autre entreprise ne voit pas le taux.
- Images produit (`image/jpeg`, `image/png`, `image/webp`) et justificatifs de dépense (ces images, plus `application/pdf`). Le client dépose les octets sur une URL signée, puis confirme. Sans `S3_BUCKET`, le dépôt est refusé et le reste de l'API fonctionne. La dépense n'accepte qu'un justificatif déjà reçu.

Commande : `go test ./internal/easystock/... -count=1`. Dernier passage : `api` 1,2 s, `commerce` 2,7 s, `media` 3,1 s, `clerkauth` 2,2 s, `integration` 17,6 s.

## Décisions

Un seul processus Commerce. Les écritures de vente GraphQL passent par gRPC. Les lectures, la création d'entreprise et les invitations utilisent le rôle RLS dans le processus API. Pas de Redis. Le prix est défini par conditionnement, pas par une multiplication silencieuse. La marge s'appelle marge brute. L'offre ne retire pas l'export : l'expiration de l'abonnement le bloque.

## Incomplet

- Mutations GraphQL encore absentes : `payDebt`, remboursement, ouverture et clôture de caisse, achats. La lecture de ces domaines est en place.
- gqlgen n'a pas pu régénérer ce schéma (erreur interne du chargeur de types avec la chaîne Go locale). L'exécution utilise `github.com/graphql-go/graphql`. L'ancien schéma gqlgen n'est pas touché.
- sqlc ne couvre que le curseur catalogue.
- L’import lit un JSON. Il n’y a plus de client MongoDB dans le dépôt.
- Pas de réconciliation Clerk périodique, pas de métriques Prometheus branchées.
- `ApplyTransitions` (essai → grâce → expiré) n’a pas de test dédié.
- Docker Compose a été démarré le 12 octobre 2026 sur cette machine. `minio/minio` et `minio/mc` ne sont plus publiés sur Docker Hub : les services utilisent `pgsty/silo` et `pgsty/mc`. Le service `roles` envoie le SQL sur l’entrée standard, car `psql -c` n’interprète pas `:'var'`. API `http://127.0.0.1:8080/health` répond 200, gRPC écoute dans le réseau interne.
- `scripts/backup.sh` a écrit `/tmp/easystock-20261012.dump` (format custom, 433 entrées, PostgreSQL 16.15). `scripts/restore.sh` n’a pas été exécuté.
- Aucune bascule de production : pas de gel des écritures, pas d’import final, pas de changement de trafic client.

## Configuration externe encore requise

Instance Clerk, secret webhook, URL JWKS, émetteur, parties autorisées, éligibilité Clerk Billing, opérateur de paiement local, mots de passe des rôles Postgres, `INTERNAL_CONTEXT_SECRET`, certificats gRPC, et éventuels identifiants S3.
