# EasyStock — Product Requirements Document

Version 1.0 • 12 octobre 2026 • Produit SaaS de gestion commerciale

## 1. Vision

EasyStock aide les commerçants à connaître ce qu’ils possèdent, ce qu’ils vendent, ce que leurs clients leur doivent et ce que contient leur caisse. Le produit démarre en RDC, notamment à Beni et Butembo, et doit pouvoir évoluer vers des entreprises possédant plusieurs boutiques.

Le produit prolonge le projet Rango Business sous une nouvelle marque : EasyStock. Les bases de travail sont une application Android Kotlin/Jetpack Compose, un backend Go/GraphQL et le passage à PostgreSQL. Clerk est retenu pour l’authentification et les organisations. Les exigences ci-dessous définissent la cible produit ; la réutilisation des fonctionnalités de Rango sera déterminée par un audit des dépôts avant développement.

Promesse : « Vos ventes, votre stock et vos dettes, au même endroit. »

## 2. Problème et public cible

Les cahiers dispersent les ventes, les dettes et les approvisionnements. Le propriétaire distingue difficilement le chiffre d’affaires, l’argent encaissé et le bénéfice. Les vendeurs peuvent vendre le même stock depuis plusieurs appareils, tandis que les connexions intermittentes compliquent la saisie.

La V1 cible les boutiques de détail, quincailleries et commerces de produits conditionnés. Les pharmacies pourront utiliser le catalogue général, mais les fonctions spécialisées de dispensation, lots et péremption sont hors V1.

| Profil | Besoin principal |
|---|---|
| Propriétaire | Connaître les ventes, les marges estimées, les dettes et la caisse |
| Responsable de boutique | Organiser son équipe et suivre son stock |
| Vendeur / caissier | Enregistrer rapidement une vente ou un encaissement |
| Magasinier | Réceptionner les marchandises et corriger les écarts autorisés |
| Comptable | Consulter et exporter les opérations financières |

Les clients des commerces sont des fiches métier et n’ont pas besoin d’un compte Clerk.

## 3. Objectifs et indicateurs

Objectifs proposés pour le pilote, à valider sur le terrain :

- 20 commerces pilotes ; au moins 10 convertis en clients payants.
- Première vente confirmée dans les 15 minutes suivant la fin de l’inscription, avec assistance possible.
- Vente de trois articles enregistrée en moins de 30 secondes par un utilisateur formé.
- Au moins 70 % des commerces activés réalisent des ventes sur trois jours distincts dans la semaine.
- Aucun doublon financier causé par la répétition d’une requête.
- Aucun accès interentreprises dans les tests d’isolation.

Suivre chaque mois : entreprises actives et payantes, conversion essai→abonnement, rétention à 30/90 jours, revenu récurrent, coût d’acquisition, coût de support, marge après frais et taux d’échec de synchronisation. Les objectifs commerciaux ne constituent pas des revenus garantis.

## 4. Périmètre V1

### Inclus

1. Authentification Clerk et création d’entreprise.
2. Boutiques, invitations, rôles et affectations.
3. Catalogue, prix, codes-barres et unités de conditionnement.
4. Approvisionnements, fournisseurs, inventaire et mouvements de stock.
5. Ventes comptant, à crédit ou avec paiement partiel.
6. Clients, dettes et remboursements de dettes.
7. Sessions de caisse, dépenses et rapprochement de clôture.
8. Tableau de bord, rapports et exports CSV.
9. Abonnement par entreprise et contrôle des quotas.
10. Cache Android, brouillons hors connexion et synchronisation contrôlée.
11. Journal d’audit, sauvegardes et administration SaaS.

### Après la V1

Marketplace publique, livraison, fidélité, intelligence artificielle, comptabilité générale réglementaire, paie, fiscalité automatisée, application iOS, ventes définitives entièrement hors connexion, lots/péremption et transferts de stock interboutiques.

La V1 supporte plusieurs boutiques indépendantes par entreprise. Chaque stock et chaque caisse appartiennent à une boutique.

## 5. Applications et navigation

**Android :** outil principal du commerçant et du personnel. Menus : Accueil, Vendre, Produits/Stock, Clients/Dettes, Caisse et Plus. Les modules non autorisés sont masqués, et le serveur contrôle les mêmes restrictions.

**Web Next.js :** portail du propriétaire pour rapports, utilisateurs, boutiques, abonnements et paramètres ; administration SaaS distincte. Éviter de dupliquer tout le point de vente Android en V1.

Le français est la langue initiale. Les libellés emploient « Vente », « Argent reçu », « Dette » et « Stock restant ». Les erreurs expliquent comment résoudre le problème et les actions destructrices montrent leurs conséquences.

## 6. Organisation, rôles et permissions

Une entreprise correspond à une organisation Clerk et à une ligne companies. Un utilisateur peut appartenir à plusieurs entreprises. L’entreprise active doit toujours être visible. Les affectations aux boutiques sont conservées dans PostgreSQL.

| Action | Propriétaire | Admin entreprise | Responsable boutique | Vendeur | Magasinier | Comptable |
|---|---|---|---|---|---|---|
| Abonnement et transfert de propriété | Oui | Non | Non | Non | Non | Non |
| Gérer les boutiques et administrateurs | Oui | Oui, hors propriétaire | Non | Non | Non | Non |
| Inviter/affecter personnel opérationnel | Oui | Oui | Dans sa boutique, sans élévation | Non | Non | Non |
| Gérer catalogue et prix | Oui | Oui | Dans son périmètre | Lecture prix vente | Lecture | Lecture |
| Réceptionner et inventorier | Oui | Oui | Oui | Non | Oui | Lecture |
| Vendre et encaisser | Oui | Oui | Oui | Oui | Non | Non |
| Autoriser crédit/remise/retour | Oui | Oui | Selon plafonds | Selon délégation | Non | Non |
| Voir coûts et marges | Oui | Oui | Selon délégation | Non | Non | Oui |
| Clôturer caisse | Oui | Oui | Oui | Sa session | Non | Lecture |
| Voir/exporter rapports | Oui | Oui | Sa boutique | Ses opérations | Son stock | Boutiques affectées |
| Audit | Entreprise | Entreprise | Sa boutique | Non | Non | Finance |

Rôles proposés : owner, organization_admin, store_manager, cashier, stockkeeper, accountant. Le catalogue exact des rôles Clerk sera configuré selon les capacités et limites du contrat retenu.

Clerk porte l’identité, les adhésions et le rôle organisationnel. PostgreSQL porte les affectations de boutique, délégations et plafonds métier. Ne pas maintenir deux sources indépendantes pour le même rôle organisationnel : son miroir local est synchronisé et réconcilié.

La décision d’accès exige simultanément : session valide, adhésion active, permission d’action, boutique autorisée et droit d’abonnement. Toute action non explicitement autorisée est refusée.

Exemples de permissions : sales.create, sales.refund, stock.receive, stock.adjust, debts.collect, cash.close, reports.read, margins.read, members.manage et billing.manage.

Le dernier propriétaire ne peut pas être supprimé. Un responsable ne peut pas se promouvoir, créer un administrateur ou étendre ses propres affectations. Les droits du support SaaS ne donnent pas un accès général aux données des commerces : toute intervention exceptionnelle est limitée, autorisée et auditée.

## 7. Parcours principaux

### Inscription

Le propriétaire crée son compte Clerk, vérifie son identité par le parcours activé, crée son entreprise, choisit sa devise principale, crée sa première boutique et ajoute ses produits. Un essai de 14 jours est proposé. L’écran d’activation montre les étapes restantes.

### Vente comptant

Le vendeur ouvre sa session de caisse, recherche ou scanne les articles, choisit les quantités et confirme le paiement. Le serveur valide les permissions, les prix et le stock, puis enregistre atomiquement vente, lignes, paiement, sorties de stock et écriture de caisse. Le reçu n’est définitif qu’après confirmation serveur.

### Vente à crédit

Le vendeur sélectionne un client obligatoire et une échéance. Le paiement reçu peut être nul ou partiel. Le montant non payé crée une créance liée à la vente. Un remboursement ultérieur réduit cette créance et crée un nouvel encaissement traçable.

### Réception

Le magasinier saisit fournisseur, produits, quantités, coût unitaire et conditionnement. La validation augmente le stock et actualise le coût moyen. La réception n’implique pas automatiquement une sortie de caisse : un paiement fournisseur est une opération distincte.

### Clôture

Le caissier saisit le montant réellement compté par devise. EasyStock compare avec le montant attendu, demande une justification de l’écart et clôture la session. La session clôturée est immuable ; les corrections créent des opérations compensatoires.

## 8. Exigences fonctionnelles

### Catalogue

Un produit possède nom, SKU, catégorie, unité de base, code-barres facultatif, seuil d’alerte et statut. Les prix sont définis par boutique et devise. Les coûts ne sont visibles qu’aux profils autorisés.

Les conditionnements utilisent un facteur explicite : par exemple un carton = 12 bouteilles. Le stock est tenu en unités de base ; le serveur effectue la conversion. Les quantités fractionnaires sont permises uniquement pour les unités configurées à cet effet. Un produit utilisé dans une opération est archivé, jamais supprimé avec son historique.

### Stock

Chaque entrée, vente, retour ou ajustement crée un mouvement daté avec auteur, motif et référence. Le solde correspond à la somme des mouvements. Les ajustements nécessitent une permission et un motif. Le stock négatif est refusé en V1.

Les brouillons ne réservent pas le stock. La validation verrouille les soldes concernés ou applique une mise à jour atomique empêchant la survente. L’inventaire compté produit un ajustement audité, sans écraser l’historique.

### Vente, annulation et retour

La vente conserve les prix, remises et coûts à la date de confirmation. Les modifications ultérieures du catalogue ne changent pas les anciennes ventes.

Une vente validée n’est ni modifiée ni supprimée. Une annulation ou un retour crée une contre-opération liée, avec contrôle des quantités déjà retournées. La réintégration en stock dépend de l’état revendable. Un remboursement réduit la créance restante et/ou restitue les paiements selon le cas ; il ne doit jamais rembourser plus que le montant encaissé net des remboursements précédents.

Les modes initiaux sont espèces et paiement externe déclaré, avec référence facultative ou obligatoire selon configuration. Enregistrer un paiement mobile money ne signifie pas le traiter : EasyStock tient la trace de l’encaissement déclaré.

### Clients et dettes

Fiche client : nom, téléphone facultatif, notes, dettes ouvertes, échéances et historique. Afficher séparément montant vendu, encaissé et restant dû. Les remboursements partiels sont autorisés. Aucun solde de dette n’est édité manuellement.

Un vendeur voit les dettes des boutiques autorisées, sans historique financier des autres boutiques. Un partage manuel de rappel est inclus ; l’envoi automatique WhatsApp est une évolution.

### Caisse et dépenses

Une caisse appartient à une boutique. En V1, une seule session est ouverte par caisse à la fois ; plusieurs caisses peuvent exister. Fonds initial, encaissements, remboursements, dépenses et autres mouvements autorisés sont distingués.

La caisse espèces n’inclut pas les encaissements mobile money ou carte. Les mouvements restent séparés par devise. Une dépense exige catégorie, montant, devise, auteur et justificatif facultatif.

### Rapports

Rapports filtrés par boutique, période et devise : ventes nettes, encaissements par mode, dettes, dépenses, stock faible, produits les plus vendus, coût des ventes et marge brute estimée.

La marge brute = ventes nettes − coût des produits vendus. Elle n’est pas présentée comme bénéfice net. Le stock initial doit avoir un coût documenté ; sinon les marges concernées sont indiquées comme incomplètes.

Ne jamais additionner CDF et USD sans conversion explicite. Les exports respectent exactement les permissions de consultation et ne contiennent pas les coûts pour un vendeur.

## 9. Abonnements et modèle économique

L’abonnement est souscrit par entreprise. Le prix et la durée changent selon l’offre. L’offre ne retire aucune fonction : l’échéance de l’abonnement est ce qui bloque l’accès. Le rôle de l’appelant continue de limiter ce qu’il peut faire. Aucune commission sur les ventes des commerçants.

| Offre proposée | Prix mensuel à tester | Fonctions |
|---|---:|---|
| Essentiel | 5 USD | Toutes, tant que l’abonnement n’est pas expiré |
| Pro | 10 USD | Toutes, tant que l’abonnement n’est pas expiré |
| Business | 20 USD | Toutes, tant que l’abonnement n’est pas expiré |

Ces prix sont des hypothèses commerciales. Déterminer les plafonds techniques du catalogue après le pilote. Ne pas bloquer arbitrairement le nombre de ventes déjà réalisées.

### Clerk Billing

Cible : abonnement B2B rattaché à l’organisation Clerk, paiement par le parcours compatible Clerk Billing, projection locale de l’état et des droits dans PostgreSQL.

Clerk Billing utilise Stripe pour le traitement des paiements et sa disponibilité dépend notamment de l’éligibilité du compte marchand. Le lancement doit valider pays d’établissement, méthodes et parcours de paiement. Il ne faut pas présumer qu’il assure l’encaissement Airtel Money ou M-Pesa en RDC.

Pour le paiement local, prévoir un adaptateur distinct : paiement externe confirmé par opérateur ou administrateur habilité, puis attribution d’un droit temporaire audité dans PostgreSQL. Ce droit est identifié comme local et n’est pas présenté comme une facture Clerk payée. Prévenir les doubles abonnements et définir quelle source finance la période active.

États normalisés : trialing, active, grace, expired et canceled. Proposition : grâce de 7 jours après échéance. Après expiration, bloquer les nouvelles opérations commerciales tout en conservant consultation et export pour les personnes autorisées. Les données ne sont pas supprimées pour impayé.

Un changement vers une offre inférieure ne supprime pas les boutiques ni utilisateurs. Le propriétaire choisit les affectations actives dans les limites du nouveau plan.

## 10. Architecture cible

| Composant | Responsabilité |
|---|---|
| Android Kotlin/Jetpack Compose | Point de vente, scan, cache Room et brouillons |
| Next.js | Portail propriétaire et administration SaaS |
| Clerk | Authentification, sessions, organisations, invitations |
| Clerk Billing | Abonnements compatibles via Stripe |
| Go + gqlgen | API GraphQL et règles métier |
| PostgreSQL + pgx/sqlc | Source de vérité métier et transactions |
| Migrations versionnées | Évolution et traçabilité du schéma |
| Stockage S3 compatible | Images et justificatifs |
| Docker Compose sur AWS | Déploiement initial |
| Redis facultatif | Cache ou limitation de débit si nécessaire |

Un monolithe modulaire Go suffit à la V1. Aucun besoin initial de Kafka, microservices ou gRPC pour ces parcours.

Le backend vérifie les jetons Clerk côté serveur : signature, émetteur, expiration et paramètres applicables du jeton. Il ne fait confiance ni à un rôle ni à un company_id simplement envoyé par le client. Les appels sensibles utilisent les permissions locales à jour, pas seulement des claims anciens.

Les événements Clerk vérifiés mettent à jour utilisateurs, organisations, adhésions et abonnements. Leur traitement doit être idempotent, supporter les reprises et éviter qu’un événement ancien écrase un état récent. Une tâche périodique réconcilie les projections avec le fournisseur.

## 11. Données et isolation PostgreSQL

Tables principales : companies, stores, users, organization_memberships, store_assignments, permission_overrides, products, product_units, product_prices, suppliers, purchases, purchase_items, stock_movements, stock_balances, customers, sales, sale_items, payments, refunds, debts, debt_payments, cash_registers, cash_sessions, cash_transactions, expenses, subscription_snapshots, local_subscription_payments, entitlement_grants, webhook_events, audit_logs et idempotency_keys.

users.clerk_user_id et companies.clerk_org_id sont uniques. Les nouvelles clés internes sont des UUID. Une table legacy_id_map conserve les références MongoDB pendant la migration.

Toutes les tables métier portent company_id ; les opérations locales portent aussi store_id. Les clés étrangères composites empêchent de rattacher une vente à un client ou produit d’une autre entreprise.

Les règles d’accès sont appliquées par le backend et renforcées par des politiques PostgreSQL RLS sur les tables concernées. Le contexte entreprise/utilisateur est défini dans chaque transaction avec une portée locale à cette transaction. Le rôle SQL de l’application ne doit pas contourner RLS. Les jobs parcourent des entreprises explicites avec le même contrôle, sans contexte global implicite.

Montants : NUMERIC avec précision et arrondis documentés, devise obligatoire, aucun FLOAT. Les décimaux sont transportés sans perte de précision dans GraphQL. Les dates sont stockées en UTC et présentées dans le fuseau de la boutique.

Chaque commande sensible possède une clé d’idempotence liée à l’entreprise, au type d’action et à son contenu. Une même clé avec un contenu différent est refusée.

## 12. Connexion intermittente

Le catalogue et les données autorisées sont mis en cache dans Room. Sans connexion, le personnel prépare des brouillons de vente. Les brouillons affichent « Non confirmé » et ne réduisent pas le stock serveur.

À la reconnexion, le serveur revérifie session, adhésion, permissions, abonnement, prix et stock. Un conflit nécessite une correction explicite ; il ne produit pas de vente silencieusement modifiée. Une requête répétée retourne le même résultat confirmé.

La première connexion nécessite internet. Déconnexion, changement d’entreprise ou retrait d’accès purge les caches sensibles selon une politique explicite. Aucun encaissement hors connexion ne doit être présenté comme une opération serveur définitivement enregistrée.

## 13. Sécurité, exploitation et qualité

- HTTPS, secrets hors dépôt, limitation de débit et validation des entrées.
- MFA exigée pour propriétaires et administrateurs ; parcours choisi selon les capacités Clerk retenues.
- Audit des changements de rôle, prix, stock, remboursements, clôtures et droits d’abonnement.
- Aucun mot de passe, jeton, secret ou donnée de carte dans les logs.
- Sauvegardes quotidiennes chiffrées et exercices de restauration avant lancement.
- Objectifs initiaux : disponibilité mensuelle 99,5 %, RPO 24 h et RTO 4 h.
- Latence cible API p95 inférieure à 1 seconde pour les lectures ordinaires et 2 secondes pour une vente, sous charge pilote définie, hors réseau client.
- Pagination de toutes les listes ; recherche par nom, SKU ou code-barres.
- Monitoring des erreurs, transactions lentes, conflits stock, webhooks et sauvegardes.

Les reçus commerciaux ne sont pas présentés comme une conformité fiscale automatique. Les éventuelles obligations de facturation sont étudiées avant d’activer une fonction fiscale spécifique.

## 14. Migration Rango → EasyStock

1. Auditer les dépôts Android et backend : fonctionnalités, contrats GraphQL, collections, index, rôles, cron, abonnements et données réelles.
2. Décider ce qui est conservé et renommer marque, documentation et configurations. Le changement d’identifiant Android dépend du mode de distribution et n’est pas automatique.
3. Créer le schéma PostgreSQL et les repositories sans modifier inutilement les contrats API.
4. Construire un import reproductible avec mapping d’identifiants, reprise et rapport de réconciliation.
5. Migrer les identités vers Clerk avec liaison explicite et vérifiée. Ne pas fusionner des comptes sur un simple email non vérifié. Les sessions changent et Android doit intégrer le nouveau parcours d’authentification.
6. Migrer organisations, adhésions, affectations et périodes d’abonnement déjà payées sans relancer de prélèvement.
7. Tester sur une copie puis comparer comptes, stocks, dettes, paiements et totaux par devise.
8. Planifier une courte fenêtre de gel des écritures, importer le delta final, valider et basculer.
9. Conserver une sauvegarde et un plan de retour. Après reprise des écritures PostgreSQL, un retour MongoDB nécessite leur réconciliation ; il ne suffit pas de changer une variable.
Le client PostgreSQL est en place. L’ancien code d’authentification locale et le client MongoDB ont été retirés de ce dépôt. L’import des données historiques se fait par un JSON, pas par une connexion à l’ancienne base.

Ne pas migrer un mot de passe en supposant son format compatible : vérifier les possibilités d’import Clerk ou prévoir une réinitialisation. Un environnement contenant seulement des données de test peut être réinitialisé après confirmation du propriétaire du projet.

## 15. Critères d’acceptation prioritaires

| Scénario | Résultat attendu |
|---|---|
| Vente de 2 articles disponibles | Vente, paiement, caisse et stock confirmés atomiquement |
| Deux vendeurs achètent la dernière unité | Une seule vente réussit ; l’autre reçoit un conflit |
| Requête répétée après timeout | Une seule vente et un seul encaissement |
| Vente à crédit 100 USD, versement 30 USD | Dette 70 USD ; encaissement 30 USD |
| Remboursement de dette 20 USD | Dette 50 USD et nouveau paiement de 20 USD |
| Retour d’article revendable | Stock réintégré et compensation financière cohérente |
| Échec au milieu d’une vente | Aucun effet partiel persisté |
| Utilisateur A demande une ressource B | Refus sans exposition des données B |
| Vendeur tente de lire une marge | Refus, y compris via export GraphQL |
| Responsable tente de devenir propriétaire | Refus audité |
| Session clôturée puis corrigée | Historique conservé et contre-opération explicite |
| Webhook reçu deux fois ou en retard | Pas de double droit ni régression d’état |
| Abonnement expiré | Nouvelles ventes refusées ; consultation/export autorisés |
| Brouillon offline avec stock épuisé | Conflit visible, aucune survente |
| Changement d’entreprise | Données et cache du bon périmètre uniquement |
| Restauration de sauvegarde | Totaux et opérations de référence retrouvés |

Ces scénarios constituent les tests d’intégration obligatoires avant le pilote, avec PostgreSQL réel et une configuration Clerk de test.

## 16. Livraison par étapes

**Étape 1 — Fondations :** audit, PostgreSQL, Clerk, isolation, boutiques et permissions.

**Étape 2 — Commerce :** catalogue, approvisionnement, stock et ventes comptant.

**Étape 3 — Finance opérationnelle :** dettes, paiements partiels, caisse, dépenses et retours.

**Étape 4 — SaaS :** rapports, web propriétaire, plans, droits et administration.

**Étape 5 — Terrain :** cache/brouillons Android, migration répétée, sécurité, restauration et pilote.

Passer à l’étape suivante uniquement lorsque les opérations critiques de la précédente sont testées. Le calendrier sera établi après l’audit, selon l’état réel de Rango.

## 17. Lancement et décisions restantes

Commencer par un groupe de 20 commerces dans une seule zone. Proposer une démonstration sur leurs propres produits, assister la première réception et la première clôture, puis mesurer l’usage pendant 14 jours. Élargir quand les stocks et caisses restent cohérents et que le support est soutenable.

À confirmer avant développement : présence de vraies données Rango ; statut de distribution Android ; éligibilité Clerk Billing/Stripe ; opérateur de paiement local ; devises et conditionnements prioritaires ; plafonds de crédit/remise ; tarifs du pilote.

## 18. Références de conception

- Projet Android d’origine : https://github.com/leenorshn/rangoapp
- Backend d’origine : https://github.com/leenorshn/rangoapp_back
- Clerk Organizations : https://clerk.com/docs/guides/organizations/overview
- Clerk Roles and Permissions : https://clerk.com/docs/guides/organizations/control-access/roles-and-permissions
- Clerk Billing : https://clerk.com/docs/guides/billing/overview

Les contraintes du fournisseur doivent être revalidées au moment de l’intégration. Les dépôts sont les sources à auditer, et non une preuve que les fonctions cibles existent déjà.
