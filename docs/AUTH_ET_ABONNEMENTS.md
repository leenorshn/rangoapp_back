# Authentification et abonnements

Le schéma exécutable est construit dans `internal/easystock/api/graphql.go` avec `graphql-go`. `internal/easystock/api/schema.graphqls` décrit le même contrat. gqlgen n'est pas utilisé.

Les montants sont des chaînes décimales. Les instants sont en UTC. Les rappels utilisent le fuseau IANA de l'entreprise (`Africa/Lubumbashi` pour Beni et Butembo). La durée d'un plan est un nombre de mois calendaires (`time.AddDate`), pas un nombre fixe de jours.

## Parcours propriétaire

1. `requestOwnerSignupOtp(phone)` envoie un code à 6 chiffres, valable 5 minutes, 5 essais, renvoi après 60 secondes. Le renvoi invalide le code précédent.
2. `verifyOwnerSignupOtp` renvoie une autorisation opaque, valable 10 minutes, à usage unique, limitée à ce numéro et à l'inscription.
3. `completeOwnerSignup` consomme cette autorisation, crée l'utilisateur, l'entreprise et le rôle `owner`. Le client ne choisit ni `phone_verified_at`, ni le rôle.

Le téléphone est normalisé en E.164 avec la région `PHONE_DEFAULT_REGION` (CD, donc `+243`) avant toute recherche. L'e-mail est facultatif et ne sert pas à se connecter.

Si le numéro existe déjà, la vérification d'inscription répond `ACCOUNT_EXISTS`. La demande de code, elle, ne révèle pas l'existence du compte.

## Employés

Seul un propriétaire actif crée un employé : nom, téléphone, rôle `manager` ou `seller`, mot de passe temporaire. `must_change_password` est vrai. Le téléphone n'est pas marqué vérifié. `OWNER` et l'administration plateforme ne peuvent pas être attribués ici.

Un numéro déjà présent est refusé (`PHONE_IN_USE`). Le compte existant n'est ni modifié, ni rattaché, et aucune autre entreprise n'est exposée.

La première connexion a le scope `password_change`. Seuls `changePassword`, `logout`, `logoutAllSessions` et `me` sont acceptés. `changePassword` révoque cette session et en ouvre une nouvelle de scope `full`. L'ancien jeton d'accès cesse immédiatement d'être valable.

`deactivateEmployee` passe l'appartenance à `disabled` et révoque les sessions de cet utilisateur pour cette entreprise. L'historique des ventes reste. `reactivateEmployee` réactive l'appartenance sans restaurer ces sessions.

L'offre ne limite pas les fonctionnalités. Boutiques, employés, rôles, marges, exports et consolidation restent disponibles tant que l'abonnement n'est pas expiré. Le rôle de l'appelant continue de limiter ce qu'il peut faire.

## Sessions

Fermer l'application ne déconnecte pas. Le jeton d'accès JWT HS256 dure 15 minutes. Il contient l'identifiant de session, pas un rôle faisant autorité. Chaque requête relit la session, l'appartenance et l'administration plateforme.

- Inactivité : 30 jours depuis le dernier renouvellement réussi.
- Durée absolue : 90 jours depuis la création. Un renouvellement ne la repousse pas.
- Le refresh token est opaque, stocké haché, et remplacé à chaque usage.
- Réutiliser un ancien refresh token révoque toute la famille. Les appels concurrents avec le même token sont une réutilisation : le client doit les sérialiser. Il n'y a pas de fenêtre de grâce.

Web (`client: "web"`) : le refresh token est le cookie persistant `easystock_refresh` (`HttpOnly`, `Secure` en production, `SameSite=Lax`, ou `None` si le cookie est marqué Secure). Le cookie `easystock_csrf` doit être renvoyé dans `X-EasyStock-CSRF`. Le refresh token n'est pas dans le JSON.

Mobile (`client` différent de `web`) : `refreshToken` est dans le JSON, à placer dans le stockage sécurisé du système.

`logout` révoque la session courante. `logoutAllSessions` révoque toutes les sessions. `mySessions` et `revokeSession` concernent uniquement les sessions de l'appelant.

CORS n'accepte que `AUTH_CORS_ORIGINS`.

## Récupération

`requestPasswordResetOtp`, `verifyPasswordResetOtp`, puis `resetPassword`. L'autorisation de récupération ne peut pas terminer une inscription, et l'inverse non plus. Le succès public de la demande de code est le même si le numéro n'a pas de compte. La réinitialisation marque le téléphone vérifié et révoque toutes les sessions. Elle ne réactive pas une appartenance désactivée.

Les mots de passe passent par bcrypt. Une entrée de plus de 72 octets est refusée, sans troncature. Rien de tout cela n'est journalisé : mot de passe, code SMS, jeton.

## Abonnement

L'abonnement appartient à l'entreprise. `lifecycle` vaut `inactive`, `active`, `expired` ou `closed`. Le statut d'une demande de paiement (`pending`, `approved`, `rejected`) est indépendant.

À partir de l'échéance réelle, même si le worker n'a pas encore tourné, les opérations métier répondent `SUBSCRIPTION_EXPIRED`. Une entreprise jamais activée répond `SUBSCRIPTION_REQUIRED`. GraphQL place ce code dans `extensions.code`. gRPC ajoute `google.rpc.ErrorInfo.reason` avec le même code.

Restent accessibles : profil, sessions, mot de passe, statut d'abonnement, instructions et déclaration de paiement pour le propriétaire, suivi du paiement. Une déclaration ne réactive pas l'entreprise. Les sessions ne sont pas révoquées seulement parce que l'abonnement a expiré.

L'essai de 14 jours déjà configuré sur l'offre Essentiel est conservé pour une nouvelle inscription. Aucun prix ni aucune période d'essai supplémentaire n'ont été inventés.

### Paiement manuel

`renewalInstructions` et `myOrganizationSubscription` fournissent l'écran. Le propriétaire voit le message, l'échéance, le montant, la durée, l'opérateur, le numéro destinataire, le bénéficiaire, la référence `ES-…` et les instructions. Un employé voit seulement : « L'abonnement de votre entreprise a expiré. Contactez le propriétaire. » Il ne reçoit ni justificatif ni coordonnées de paiement.

Si une demande est `pending` : « Votre paiement est en cours de vérification », `canDeclarePayment` est faux, et les opérations métier restent bloquées.

`submitSubscriptionPayment` enregistre la référence et, facultativement, un justificatif déjà déposé (5 Mo, jpeg, png ou pdf). Un administrateur plateforme (`users.is_platform_admin`, posé en SQL, jamais par `createEmployee`) appelle `approveSubscriptionPayment` ou `rejectSubscriptionPayment`.

L'approbation est transactionnelle. Le montant reçu doit égaler le montant attendu, dans la devise du plan. La même référence externe ne finance qu'une demande pour un couple opérateur et compte destinataire. Une seconde approbation de la même demande renvoie l'échéance déjà calculée.

Nouvelle échéance : `début = max(échéance actuelle, instant d'approbation)`, puis `début` plus les mois calendaires du plan. Le mobile relit `myOrganizationSubscription` sans se reconnecter.

Les instructions se configurent avec `upsertPaymentInstructions`. Aucun numéro de paiement n'est codé en dur. `updateSubscriptionPlan` change le prix mensuel existant et `duration_months`.

## Rappels

`easystock worker` tourne toutes les 5 minutes. Plusieurs instances s'appuient sur les contraintes d'unicité et `FOR UPDATE SKIP LOCKED`.

- J-3 à 09:00 locale : notification mobile et bannière.
- J-1 à 09:00 : SMS, mobile et bannière.
- Après l'échéance : SMS et mobile à 08:00 et 18:00, heure de l'entreprise, jusqu'au renouvellement.
- L'échéance qui tombe entre 08:00 et 18:00 envoie son SMS sur le créneau du matin s'il est déjà passé, ou à l'heure d'échéance. Ce SMS compte dans la limite de deux SMS de rappel par jour.
- Une échéance après 18:00 bloque tout de suite et attend le lendemain 08:00 pour le SMS.
- Une demande `pending` suspend les rappels de paiement. Après un rejet, seuls les créneaux futurs partent.
- Une entreprise `closed` n'est pas relancée.
- `updateReminderPreferences` coupe les SMS de rappel, pas les SMS de sécurité.
- Les employés ne reçoivent pas les SMS de facturation.

La bannière est calculée depuis l'état courant, même si le push échoue. Un créneau est encore pertinent pendant 2 heures (`reminders.RelevanceWindow`). Au-delà, il n'est pas rattrapé : une panne ne produit pas de rafale.

Avant l'envoi, le worker revérifie l'échéance, l'absence de paiement en attente, la préférence SMS et le propriétaire actif. Un timeout ambigu de la passerelle passe la ligne en `unknown` et n'est pas renvoyé automatiquement. Le fournisseur HTTP reçoit `Idempotency-Key`. Cinq essais au plus, avec un backoff, pour les erreurs transitoires. Ce n'est pas une garantie d'exactement un SMS si la passerelle ne sait pas résoudre un timeout.

Une demande en attente depuis plus de 24 heures alerte l'administration dans l'application. Cela ne crée pas un nouveau paiement.

## Codes stables

| Code | Situation |
|---|---|
| `OTP_INVALID` | Code incorrect |
| `OTP_EXPIRED` | Code expiré |
| `OTP_ATTEMPTS` | Cinq essais atteints |
| `OTP_RESEND` | Renvoi avant 60 secondes |
| `ACCOUNT_EXISTS` | Numéro déjà inscrit, après preuve du code |
| `PHONE_IN_USE` | Employé refusé, compte existant inchangé |
| `MUST_CHANGE_PASSWORD` | Session limitée au changement de mot de passe |
| `SUBSCRIPTION_EXPIRED` | Échéance dépassée |
| `SUBSCRIPTION_REQUIRED` | Abonnement absent, inactif ou entreprise fermée |
| `PAYMENT_PENDING` | Une déclaration est déjà en vérification |
| `rate_limited` | Trop de demandes |
| `csrf` | Cookie web sans jeton CSRF correspondant |

## Exemples

Les valeurs ci-dessous sont fictives. Ne pas y placer un secret, un code reçu ou un refresh token réel.

```graphql
mutation {
  requestOwnerSignupOtp(phone: "+243970000001")
}
```

```graphql
mutation {
  verifyOwnerSignupOtp(phone: "+243970000001", code: "000000")
}
```

```graphql
mutation {
  completeOwnerSignup(input: {
    grant: "<autorisation>",
    phone: "+243970000001",
    fullName: "Amina",
    password: "<mot de passe>",
    companyName: "Beni Market",
    primaryCurrency: "USD",
    timezone: "Africa/Lubumbashi",
    client: "mobile"
  }) {
    accessToken
    refreshToken
    scope
    companyId
  }
}
```

```graphql
mutation {
  login(input: { phone: "+243970000001", password: "<mot de passe>", client: "web" }) {
    accessToken
    csrfToken
    scope
    mustChangePassword
  }
}
```

Le client web envoie ensuite `Authorization: Bearer <accessToken>` et `X-EasyStock-CSRF`, avec les cookies. `refreshSession` sans argument relit le cookie. Le mobile passe `refreshToken`.

```graphql
query {
  renewalInstructions {
    message
    expiresAt
    amount
    currency
    durationMonths
    renewalReference
    operatorName
    destinationAccount
    beneficiaryName
    paymentInstructions
    paymentStatus
    canDeclarePayment
  }
}
```

## Migration depuis Clerk

La migration `000005_internal_auth` ne supprime ni utilisateurs, ni entreprises, ni appartenances, ni ventes. Les colonnes `clerk_user_id`, `clerk_org_id` et `clerk_membership_id` restent pour l'historique.

- Aucun mot de passe n'est fabriqué. Un compte Clerk sans `password_hash` a `must_change_password`.
- `phone_verified_at` n'est pas rempli sans une preuve SMS ultérieure.
- Les numéros vides, doublons et non E.164 sont copiés dans `phone_migration_issues` (`missing`, `duplicate`, `not_e164`). Les doublons et les numéros vides sont retirés de `users.phone` pour permettre l'unicité. Les comptes ne sont pas fusionnés.
- Ces utilisateurs ne peuvent pas se connecter tant que le numéro n'est pas corrigé puis prouvé par le parcours SMS de récupération, qui fixe ensuite le mot de passe.

Les clients doivent cesser d'envoyer des jetons Clerk. `POST /webhooks/clerk` n'est plus servi. Les sessions Clerk sont donc invalides dès le déploiement : il n'existe pas de table de sessions Clerk à révoquer, et les JWT Clerk ne sont plus vérifiés. Le paquet `internal/easystock/clerkauth` reste uniquement pour ses tests historiques.

Après déploiement, un propriétaire dont le numéro est unique et déjà en E.164 utilise la récupération SMS pour choisir son mot de passe. Un cas listé dans `phone_migration_issues` doit être résolu à la main avant cela.

## Configuration

| Variable | Rôle |
|---|---|
| `AUTH_JWT_SECRET` | Signature des jetons d'accès, 32 caractères au minimum |
| `AUTH_OTP_SECRET` | HMAC des codes SMS, distinct du précédent |
| `PHONE_DEFAULT_REGION` | Région des numéros locaux, `CD` par défaut |
| `APP_ENV` | `production` interdit les fournisseurs simulés |
| `SMS_PROVIDER` | `dev` en local, `uzashop` ou `http` en production |
| `SMS_HTTP_URL` | `POST` JSON `{"to","body","idempotency_key"}` lorsque `SMS_PROVIDER=http` |
| `SMS_HTTP_TOKEN` | Bearer envoyé à la passerelle HTTP, si elle en exige un |
| `SMS_UZA_TOKEN` | `tokenKey` de l'abonnement [UZASHOP SMS Management V2](https://sms.uzashop.co/api/documentation#/SMS%20Management%20V2), requis si `SMS_PROVIDER=uzashop` |
| `SMS_UZA_SENDER` | Nom d'expéditeur, 11 caractères au plus. Défaut `EasyStock` |
| `SMS_UZA_BASE_URL` | Origine de l'API. Défaut `https://sms.uzashop.co`. L'envoi appelle `POST /api/v2/sms/send` |
| `PUSH_PROVIDER` | `dev` ou `http` |
| `PUSH_HTTP_URL` | `POST` JSON `{"token","title","body","idempotency_key"}` |
| `PUSH_HTTP_TOKEN` | Bearer de la passerelle push |
| `REDIS_URL` | Limites de débit. Vide : compteurs PostgreSQL |
| `AUTH_CORS_ORIGINS` | Origines web, séparées par des virgules |
| `AUTH_COOKIE_SECURE` | `true` force le cookie Secure |

`SMS_PROVIDER=dev` journalise « SMS simulé, non envoyé » et n'émet rien. Ce n'est pas un SMS délivré. Le même avertissement vaut pour le push.

`SMS_PROVIDER=uzashop` envoie les codes et les rappels via `POST /api/v2/sms/send`. Le corps contient `tokenKey`, `phoneNumber` (E.164 sans le `+`), `senderName` et `message` (160 caractères au plus). Un délai dépassé reste ambigu et n'est pas renvoyé. Un solde insuffisant, un abonnement inactif ou une validation refusée n'est pas retenté tout de suite. L'adaptateur `http` reste disponible pour une autre passerelle.

Le numéro `+243978154329` (ou `0978154329`) accepte toujours le code `123123` après une demande d'OTP, y compris sur le déploiement cloud. Aucun SMS n'est envoyé pour ce numéro. L'expiration, le délai de renvoi et le nombre d'essais restent en vigueur. Les autres numéros reçoivent un code aléatoire.

Le justificatif de paiement réutilise le stockage S3 déjà configuré (`S3_*`). Sans stockage, une déclaration avec `proofKey` est refusée. Une déclaration sans justificatif reste possible.

Pour nommer un administrateur plateforme, après qu'il a un compte :

```sql
UPDATE users SET is_platform_admin = true WHERE phone = '+243970000001';
```

Aucun administrateur ni aucun numéro de paiement n'est créé par les migrations.
