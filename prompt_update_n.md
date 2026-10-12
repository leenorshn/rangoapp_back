Tu es un ingénieur backend senior spécialisé en Go, PostgreSQL, GraphQL, gRPC et applications SaaS multi-entreprises.

MISSION
Mettre à jour le backend existant de Rango Business pour devenir EasyStock, une plateforme de gestion commerciale : produits, stock, ventes, clients, dettes, caisse, approvisionnements et abonnements.

Dépôt concerné :
https://github.com/leenorshn/rangoapp_back

Travaille directement sur le projet existant. Analyse le code avant de modifier les composants. Réutilise les fonctionnalités correctes et conserve les contrats GraphQL autant que possible.

Ne te limite pas à des recommandations : implémente les changements par étapes, avec migrations, tests et documentation.

1. STACK CIBLE

- Go.
- gqlgen / GraphQL pour les applications clientes.
- gRPC pour la communication interne entre services.
- PostgreSQL comme source de vérité métier.
- pgx/v5 + sqlc pour les accès SQL.
- Migrations SQL versionnées.
- Clerk pour l’authentification, les sessions et les organisations.
- Clerk Billing pour les abonnements compatibles.
- Adaptateur distinct pour les paiements locaux d’abonnement.
- Stockage compatible S3 pour les images et justificatifs.
- Docker Compose pour le développement et le déploiement initial.
- Redis uniquement lorsqu’un besoin concret le justifie.

L’authentification est Clerk. PostgreSQL est la seule base du dépôt.

2. AUDIT INITIAL

Avant toute modification fonctionnelle :

- Lire les instructions du dépôt.
- Examiner go.mod, l’arborescence, les points d’entrée et Docker.
- Examiner le schéma PostgreSQL, les migrations, les index et les politiques RLS.
- Examiner schémas GraphQL, resolvers, services et middlewares.
- Identifier les transactions financières et mouvements de stock.
- Examiner authentification, rôles, abonnements, cron et stockage.
- Identifier les dépendances de l’application Android.
- Repérer les fonctionnalités existantes à conserver.
- Ne jamais afficher de secrets ou de données sensibles.

Produire :
- EASYSTOCK_BACKEND_AUDIT.md
- EASYSTOCK_IMPLEMENTATION_PLAN.md
- EASYSTOCK_POSTGRES_SCHEMA.md
- EASYSTOCK_GRPC_ARCHITECTURE.md

Distinguer clairement ce qui existe, ce qui manque et ce qui doit être corrigé.

Après l’audit, poursuivre l’implémentation sans attendre une confirmation supplémentaire, sauf décision réellement bloquante ou opération destructive.

3. ARCHITECTURE ET FRONTIÈRES gRPC

Conserver une architecture simple adaptée au projet.

Structurer les domaines :
- identity : projection Clerk, entreprises et adhésions.
- access : affectations aux boutiques et permissions.
- catalog : produits, conditionnements et prix.
- inventory : réceptions, inventaires et mouvements.
- sales : ventes, retours et remboursements.
- customers : clients et dettes.
- cash : sessions, encaissements et dépenses.
- reports : tableaux de bord et exports.
- subscriptions : abonnements et droits.
- media : images et justificatifs.

Ne pas transformer automatiquement chaque domaine en microservice.

Garder ventes, stock, dettes, paiements et caisse dans un même service transactionnel Commerce afin de préserver les transactions PostgreSQL atomiques.

Créer des contrats gRPC pour les frontières internes utiles, notamment :
- CommerceService.
- SubscriptionService.
- MediaService, si ce service existe ou si son extraction est justifiée.

Le processus API GraphQL peut appeler CommerceService par gRPC. Les modules Commerce continuent à communiquer directement en Go à l’intérieur du même processus transactionnel.

Si l’architecture existante est déjà distribuée, adapter ces frontières et documenter la stratégie de cohérence. Ne jamais simuler une transaction distribuée avec plusieurs appels gRPC supposés atomiques.

4. CONTRATS ET INFRASTRUCTURE gRPC

Utiliser :
- google.golang.org/grpc.
- Protocol Buffers.
- Un package versionné, par exemple easystock.commerce.v1.
- Une génération reproductible des clients et serveurs.
- Buf ou les outils de génération déjà présents.

Exigences :
- Aucun accès public direct aux ports gRPC internes.
- Authentification des services ; mTLS en production.
- Identité utilisateur et entreprise transmises dans un contexte vérifiable.
- Ne pas faire confiance à company_id, store_id ou user_id reçus seuls dans les métadonnées.
- Interceptors pour authentification, autorisation, logs, récupération des paniques et suivi des requêtes.
- Deadlines explicites propagées aux accès PostgreSQL.
- Annulation propagée avec context.Context.
- Limites de taille des messages.
- Health-check standard gRPC.
- Arrêt gracieux.
- Réutilisation des connexions clients.
- Pas de retry automatique des commandes non idempotentes.
- Aucun streaming ajouté sans besoin produit.

Utiliser les codes gRPC appropriés :
Unauthenticated, PermissionDenied, InvalidArgument,
NotFound, AlreadyExists, FailedPrecondition,
Aborted, ResourceExhausted, Unavailable et Internal.

Ne jamais renvoyer de détails SQL sensibles aux clients.

Représenter :
- Les montants comme décimaux exacts, jamais float/double.
- Les identifiants comme chaînes UUID.
- Les dates avec google.protobuf.Timestamp.
- Les listes avec pagination.

Les champs supprimés des .proto doivent être réservés. Ne pas réutiliser leurs numéros.

5. POSTGRESQL

Créer les tables nécessaires :

Organisation :
companies, stores, users, organization_memberships,
store_assignments, permission_overrides.

Catalogue :
products, product_units, product_prices.

Approvisionnement et stock :
suppliers, purchases, purchase_items,
stock_movements, stock_balances.

Commerce :
customers, sales, sale_items, payments,
refunds, refund_items, debts, debt_payments.

Caisse :
cash_registers, cash_sessions,
cash_transactions, expenses.

SaaS :
subscription_snapshots, local_subscription_payments,
entitlement_grants.

Exploitation :
webhook_events, audit_logs,
idempotency_keys, legacy_id_map.

Adapter ces noms au code existant si cela réduit les risques, sans perdre les responsabilités.

Règles :
- UUID pour les nouvelles clés.
- clerk_user_id et clerk_org_id uniques.
- company_id sur les données métier.
- store_id pour les données propres à une boutique.
- NUMERIC pour montants et quantités décimales.
- Devise explicite.
- Dates stockées en UTC.
- Contraintes et index cohérents avec les requêtes.
- Clés étrangères empêchant les références interentreprises.
- Historique conservé : archiver plutôt que supprimer les ressources utilisées.

6. ISOLATION MULTI-TENANT

Chaque requête doit vérifier :
- Session valide.
- Adhésion active à l’entreprise.
- Permission métier.
- Affectation à la boutique.
- Droit d’abonnement applicable.

L’entreprise active vient du contexte authentifié et validé.

Appliquer les contrôles dans les services métier, pour les appels GraphQL comme gRPC.

Renforcer l’isolation avec PostgreSQL RLS :
- Contexte utilisateur/entreprise défini dans chaque transaction.
- Utiliser SET LOCAL ou set_config avec portée transactionnelle.
- Aucun contexte tenant persistant sur une connexion du pool.
- Rôle SQL applicatif sans BYPASSRLS.
- Éviter le contournement par propriété des tables, notamment avec FORCE ROW LEVEL SECURITY lorsque nécessaire.
- Séparer rôle de migration et rôle applicatif.
- Jobs et webhooks avec contexte explicite et permissions limitées.

Un utilisateur d’une entreprise ne doit jamais consulter ou modifier les données d’une autre, même en devinant ses UUID.

7. CLERK ET MIGRATION DE L’AUTHENTIFICATION

Remplacer l’authentification locale par Clerk.

Le backend vérifie les jetons côté serveur :
- Signature.
- Émetteur.
- Expiration.
- Paramètres applicables au type de jeton utilisé.
- Organisation et adhésion.
- Permissions métier locales à jour.

Utiliser les mécanismes officiellement supportés par Clerk et vérifier la documentation actuelle.

Clerk est la source de vérité pour :
- Identité.
- Sessions.
- Organisations.
- Adhésions.
- Rôle organisationnel.

PostgreSQL conserve :
- Projection des données Clerk.
- Affectations de boutiques.
- Délégations.
- Plafonds de remise et crédit.
- Audit.
- Données métier.

Ne pas maintenir deux sources indépendantes pour le même rôle.

Traiter les webhooks Clerk :
- Signature vérifiée sur le corps brut.
- Idempotence.
- Reprise sur erreur.
- Protection contre événements anciens.
- Réconciliation périodique.
- Aucun secret dans les logs.

Prévoir une migration vérifiable des comptes existants :
- Mapping explicite.
- Pas de fusion sur un email non vérifié.
- Vérifier la compatibilité d’import des mots de passe.
- Sinon prévoir réinitialisation.
- Documenter les changements nécessaires côté Android.

8. RÔLES ET PERMISSIONS

Rôles cibles :
- owner.
- organization_admin.
- store_manager.
- cashier.
- stockkeeper.
- accountant.

Configurer leur correspondance avec les capacités Clerk disponibles.

Permissions :
- sales.create.
- sales.refund.
- sales.discount.
- credit.authorize.
- stock.receive.
- stock.adjust.
- debts.collect.
- cash.open.
- cash.close.
- expenses.create.
- reports.read.
- reports.export.
- margins.read.
- members.manage.
- stores.manage.
- billing.manage.

Règles :
- Le dernier propriétaire ne peut pas être supprimé.
- Seul le propriétaire gère transfert de propriété et abonnement.
- Un responsable ne peut pas se promouvoir.
- Un vendeur ne voit pas les coûts ou marges sans permission.
- Les droits sont limités aux boutiques affectées.
- Les exports appliquent les mêmes restrictions.
- L’administration SaaS n’accorde pas un accès universel aux commerces.
- Toute intervention de support sur des données métier est limitée et auditée.

9. LOGIQUE COMMERCIALE

Catalogue :
- Produits avec SKU, catégorie, unité et code-barres facultatif.
- Prix par boutique et devise.
- Conditionnements explicites : carton de 12 = 12 unités.
- Quantités fractionnaires uniquement pour les unités autorisées.

Stock :
- Historique des mouvements.
- Aucun stock négatif en V1.
- Validation concurrente empêchant la survente.
- Ajustements avec motif et permission.
- Inventaires produisant des mouvements correctifs.
- Brouillons sans réservation de stock.

Vente :
Enregistrer dans une même transaction PostgreSQL :
- Vente.
- Lignes et prix figés.
- Coûts figés nécessaires à la marge.
- Paiements.
- Sorties de stock.
- Dette éventuelle.
- Écritures de caisse.
- Audit.

Une erreur annule tous les effets de cette transaction.

Dette :
- Client obligatoire pour une vente à crédit.
- Paiements partiels.
- Remboursements traçables.
- Aucun solde modifié arbitrairement.
- Pas de remboursement supérieur au solde restant.

Retours :
- Pas de suppression d’une vente confirmée.
- Contre-opérations liées à la vente.
- Contrôle des quantités déjà retournées.
- Réintégration du stock uniquement si revendable.
- Compensation cohérente de la dette et des paiements.
- Aucun remboursement supérieur aux montants encaissés nets.

Caisse :
- Une seule session ouverte par caisse.
- Plusieurs caisses possibles par boutique.
- Fonds initial, entrées, sorties et écarts distincts.
- Espèces séparées des autres moyens de paiement.
- Soldes séparés par devise.
- Clôture immuable et corrections compensatoires.

Approvisionnement :
- Réception et paiement fournisseur distincts.
- Coût moyen documenté et cohérent.
- Stock initial avec coût ; signaler les marges incomplètes sinon.

10. IDEMPOTENCE

Les commandes sensibles GraphQL et gRPC acceptent une clé d’idempotence.

La clé est liée à :
- Entreprise.
- Type de commande.
- Empreinte du contenu.

Garantir par contrainte SQL et transaction :
- Même clé et même contenu : même résultat.
- Même clé et contenu différent : refus.
- Requêtes simultanées : un seul effet métier.
- Reprise après timeout : pas de double vente ou paiement.

Ne pas se limiter à un cache Redis pour protéger les opérations financières.

11. ABONNEMENTS

Abonnement par entreprise, sans commission sur les ventes.

Offres configurables proposées :
- Essentiel : 5 USD/mois, 1 boutique, 2 collaborateurs.
- Pro : 10 USD/mois, 1 boutique, 5 collaborateurs.
- Business : 20 USD/mois, 3 boutiques, 15 collaborateurs.

Essai proposé : 14 jours.
Grâce proposée : 7 jours.

États normalisés :
trialing, active, grace, expired, canceled.

Vérifier l’éligibilité réelle à Clerk Billing et ses moyens de paiement. Ne pas supposer qu’il prend en charge Airtel Money ou M-Pesa.

Prévoir un adaptateur local :
- Paiement externe confirmé.
- Validation par opérateur ou administrateur habilité.
- Attribution d’un droit limité dans le temps.
- Audit.
- Protection contre les doubles activations.
- Source du droit clairement identifiée.
- Aucune facture Clerk marquée payée artificiellement.

À expiration :
- Refuser les nouvelles opérations commerciales.
- Conserver consultation et export autorisés.
- Ne pas supprimer les données.

Une réduction d’offre ne supprime pas automatiquement boutiques ou utilisateurs.

12. RAPPORTS

Implémenter :
- Ventes nettes.
- Encaissements par moyen.
- Dettes et échéances.
- Dépenses.
- Stock faible.
- Produits les plus vendus.
- Coût des ventes.
- Marge brute estimée.
- Rapport de caisse.
- Consolidation des boutiques autorisées.

Ne pas appeler la marge brute « bénéfice net ».
Ne pas additionner CDF et USD sans conversion explicite.
Pagination, filtres et exports sécurisés obligatoires.

13. SUPPORT ANDROID ET CONNEXION INTERMITTENTE

Le backend doit permettre :
- Synchronisation paginée du catalogue.
- Identification des modifications et archives.
- Brouillons de vente.
- Confirmation avec idempotence.
- Détection explicite des conflits.

À la confirmation, revérifier :
session, permissions, abonnement, stock et prix.

Un brouillon hors connexion n’est pas une vente définitive.
Ne pas confirmer silencieusement une vente dont les prix ou quantités ont changé.

Documenter les changements GraphQL nécessaires au client Android sans modifier ce dépôt Android dans cette mission.

14. DOCKER ET EXPLOITATION

Mettre à jour Docker Compose :
- PostgreSQL avec volume persistant.
- API GraphQL.
- Services gRPC nécessaires.
- Réseau interne pour PostgreSQL et gRPC.
- Aucun port gRPC exposé publiquement par défaut.
- Health-checks.
- Dépendances de démarrage.
- Arrêt gracieux.
- Variables d’environnement documentées.
- .env.example sans secrets réels.

Prévoir :
- Sauvegarde quotidienne.
- Procédure de restauration testée.
- Logs structurés.
- Identifiant de requête propagé GraphQL → gRPC → SQL.
- Métriques d’erreurs, latence, webhooks et conflits.
- Timeouts et limites de connexion.

15. TESTS OBLIGATOIRES

Avec PostgreSQL réel, vérifier :
- Vente atomique.
- Rollback complet sur erreur.
- Deux ventes concurrentes de la dernière unité.
- Idempotence concurrente.
- Vente à crédit et paiements partiels.
- Retours et remboursements.
- Clôture de caisse.
- Isolation interentreprises.
- Isolation entre boutiques.
- RLS avec le rôle SQL réel de l’application.
- Absence de fuite de contexte dans le pool.
- Restrictions coûts/marges dans API et exports.
- Webhooks répétés et désordonnés.
- Expiration d’abonnement.
- Quotas.
- Migration et réconciliation.

Pour gRPC :
- Service non authentifié refusé.
- Contexte utilisateur falsifié refusé.
- Deadlines et annulations propagées.
- Codes d’erreur cohérents.
- Health-check.
- Pas de double opération après retry.
- Arrêt gracieux.

16. MIGRATION DES DONNÉES

L’import JSON vers PostgreSQL existe déjà (`easystock import`) :
- Mode dry-run.
- Mapping d’identifiants.
- Import reproductible.
- Reprise sur erreur.
- Rapport de réconciliation.
- Totaux comparés par entreprise et devise.
- Vérification stocks, ventes, paiements et dettes.
- Journal des anomalies sans données sensibles.

Ne jamais exécuter une migration destructive ou une bascule de production sans instruction explicite.

Préparer une procédure de gel des écritures, import final et retour arrière. Après de nouvelles écritures PostgreSQL, un rollback nécessite leur réconciliation.

17. ORDRE D’IMPLÉMENTATION

Phase 1 : audit et architecture.
Phase 2 : PostgreSQL et isolation.
Phase 3 : Clerk, organisations et permissions.
Phase 4 : noyau transactionnel Commerce.
Phase 5 : contrats et communication gRPC.
Phase 6 : abonnements et rapports.
Phase 7 : compatibilité Android et synchronisation.
Phase 8 : migration, Docker et vérification finale.

Chaque phase doit produire du code utilisable et des tests pertinents. Ne pas déclarer une fonctionnalité terminée si elle contient un stub critique.

18. LIVRABLES FINAUX

- Backend EasyStock fonctionnel.
- Schéma et migrations PostgreSQL.
- Requêtes sqlc et repositories.
- Intégration Clerk.
- Contrats .proto et génération.
- Serveurs et clients gRPC.
- Contrats GraphQL compatibles ou changements documentés.
- Tests critiques.
- Docker Compose.
- Outil de migration.
- README mis à jour.
- EASYSTOCK_MIGRATION_RUNBOOK.md.
- EASYSTOCK_ANDROID_API_CHANGES.md.
- EASYSTOCK_IMPLEMENTATION_STATUS.md.

Dans le compte rendu final, préciser :
- Ce qui a été réellement implémenté.
- Les tests exécutés et leurs résultats.
- Les changements incompatibles.
- Les configurations externes restant à effectuer.
- Les risques ou fonctions encore incomplètes.

Commence maintenant par inspecter le dépôt et produire l’audit, puis exécute le plan par étapes.