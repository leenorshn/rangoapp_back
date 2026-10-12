# Architecture gRPC EasyStock

## Forme

Monolithe modulaire. Trois processus au plus en déploiement initial :

| Processus | Rôle | Port publié |
|---|---|---|
| `easystock api` | GraphQL, webhooks Clerk, média HTTP | 8080 |
| `easystock grpc` | CommerceService et SubscriptionService | aucun |
| PostgreSQL | Source de vérité | 127.0.0.1:5432 en développement seulement |

Les modules catalogue, stock, ventes, dettes et caisse s’appellent en Go dans le processus gRPC et partagent une transaction PostgreSQL. L’API n’écrit pas ces agrégats en SQL direct : elle appelle gRPC.

`MediaService` n’est pas un troisième service réseau. Les métadonnées média sont des lignes PostgreSQL ; le fichier va sur disque local ou sur un dépôt S3 compatible depuis le processus API. L’extraire n’apporte pas de transaction métier.

## Contrats

- `easystock.commerce.v1.CommerceService`
- `easystock.subscription.v1.SubscriptionService`
- Santé standard `grpc.health.v1`

Packages Go générés sous `internal/easystock/gen`. Montants en chaînes décimales, identifiants UUID, dates `google.protobuf.Timestamp`, listes paginées. Les numéros de champs retirés seront réservés.

## Confiance

L’API vérifie le jeton Clerk, charge l’adhésion en base, puis signe un contexte interne HMAC (utilisateur, entreprise, rôle, requête, expiration courte, audience `easystock-commerce`).

Le serveur gRPC :

- refuse l’appel sans ce contexte (`Unauthenticated`) ;
- refuse un `company_id` de métadonnée qui ne correspond pas au contexte signé (`PermissionDenied`) ;
- ne lit pas l’entreprise uniquement depuis les métadonnées ;
- active TLS mutuel lorsque `GRPC_TLS_CERT`, `GRPC_TLS_KEY` et `GRPC_TLS_CLIENT_CA` sont définis ;
- en développement, n’écoute que `127.0.0.1` si `GRPC_INSECURE_DEV=true`.

Intercepteurs : authentification, journal (méthode, code, durée, identifiant de requête, pas de corps), récupération de panique, deadline. Le contexte annulé est transmis à PostgreSQL. Taille max des messages : 1 Mio. Pas de retry client sur les commandes. La reprise sûre est la clé d’idempotence SQL. Connexion gRPC créée une fois et réutilisée. Arrêt par `GracefulStop`.

## Codes

| Situation | Code |
|---|---|
| Contexte absent ou expiré | Unauthenticated |
| Rôle, boutique ou contexte falsifié | PermissionDenied |
| Corps invalide | InvalidArgument |
| Ressource absente dans le périmètre | NotFound |
| Doublon | AlreadyExists |
| Abonnement expiré, caisse fermée, clé d’idempotence divergente | FailedPrecondition |
| Conflit de stock ou de prix | Aborted |
| Message trop grand | ResourceExhausted |
| PostgreSQL injoignable | Unavailable |
| Panique ou erreur non classée | Internal |

Les messages renvoyés ne contiennent ni SQL, ni secret, ni donnée d’une autre entreprise.
