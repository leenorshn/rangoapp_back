# RangoApp Backend - GraphQL Server

Serveur backend GraphQL en Golang utilisant gqlgen et MongoDB Atlas pour une application de gestion multi-points-de-vente (RangoApp).

## Architecture

- **Langage**: Golang (Go 1.22+)
- **Framework GraphQL**: gqlgen
- **Base de données**: MongoDB Atlas
- **Authentification**: JWT (JSON Web Tokens)
- **Structure**: Architecture propre avec séparation des couches (resolvers, services, repositories, models)

## Architecture Multi-Boutiques

- Une **Company** (entreprise) peut avoir plusieurs **Stores** (boutiques/magasins)
- Chaque **Store** a ses propres produits, clients, factures, fournisseurs et rapports de stock
- Un utilisateur **Admin** peut accéder à toutes les boutiques de son entreprise
- Un utilisateur **User** (non-admin) peut être affecté à une seule boutique spécifique
- Lors de la création d'une Company, un utilisateur Admin et un premier Store sont créés automatiquement

## Installation

1. Cloner le repository
2. Installer les dépendances:
```bash
go mod download
```

3. Créer un fichier `.env` basé sur `.env.example`:
```bash
cp .env.example .env
```

4. Configurer les variables d'environnement dans `.env`

5. Générer les modèles GraphQL:
```bash
go run github.com/99designs/gqlgen generate
```

6. Lancer le serveur:
```bash
go run server.go
```

Le serveur sera accessible sur `http://localhost:8080` avec le GraphQL Playground sur `http://localhost:8080/`

## Structure du Projet

```
rangoapp_back/
├── database/          # Repositories MongoDB
│   ├── connect.go
│   ├── user_db.go
│   ├── company_db.go
│   ├── store_db.go
│   ├── product_db.go
│   ├── client_db.go
│   ├── provider_db.go
│   ├── facture_db.go
│   └── rapport_store_db.go
├── graph/            # Schéma GraphQL et resolvers
│   ├── schema.graphqls
│   ├── schema.resolvers.go
│   └── resolver.go
├── middlewares/      # Middlewares HTTP
│   └── auth.go
├── services/         # Services avec logique métier
│   └── auth_service.go
├── utils/            # Utilitaires (JWT, password)
│   ├── jwt.go
│   └── password.go
├── directives/       # Directives GraphQL
│   └── auth_directive.go
└── server.go        # Point d'entrée
```

## Fonctionnalités

### Authentification
- **Register**: Création de compte avec création automatique de Company, Admin User et premier Store
- **Login**: Authentification par phone et mot de passe
- **JWT**: Génération de tokens JWT avec companyId, role, storeIds

### Gestion des Utilisateurs
- CRUD complet pour les utilisateurs
- Gestion des rôles (Admin, User)
- Blocage/Déblocage d'utilisateurs
- Affectation des utilisateurs aux stores

### Gestion de l'Entreprise
- Création automatique lors de l'inscription
- Mise à jour de l'entreprise (seulement pour Admin)

### Gestion des Stores
- CRUD complet pour les stores (seulement pour Admin)
- Création automatique du premier store lors de l'inscription

### Gestion des Produits
- CRUD complet pour les produits
- Gestion du stock au niveau du Store
- Validation des prix (prix de vente >= prix d'achat)

### Gestion des Clients
- CRUD complet pour les clients
- Association des clients à un Store spécifique

### Gestion des Fournisseurs
- CRUD complet pour les fournisseurs
- Association des fournisseurs à un Store spécifique

### Gestion des Factures
- Création de factures avec plusieurs produits
- Génération automatique du numéro de facture (unique par Store)
- Mise à jour automatique du stock

### Gestion des Rapports de Stock
- Création de rapports d'entrée/sortie de stock
- Mise à jour automatique du stock

## Règles Métier

1. **Isolation Multi-Niveaux**:
   - Toutes les données appartiennent à une Company
   - Les données opérationnelles appartiennent à un Store spécifique
   - Un **Admin** peut accéder à tous les stores de sa Company
   - Un **User** ne peut accéder qu'au store auquel il est assigné

2. **Sécurité**:
   - Tous les endpoints (sauf login/register) doivent être protégés par JWT
   - Vérification que l'utilisateur a accès au Store concerné avant toute opération

## Variables d'Environnement

Voir `.env.example` pour la liste complète des variables d'environnement requises.

## Prochaines Étapes

1. Générer les modèles GraphQL avec `go run github.com/99designs/gqlgen generate`
2. Implémenter tous les resolvers dans `schema.resolvers.go`
3. Ajouter des tests unitaires et d'intégration
4. Ajouter la pagination pour les listes
5. Ajouter la validation des entrées utilisateur

## Notes

- Les modèles GraphQL doivent être régénérés après chaque modification du schéma
- Les index MongoDB sont créés automatiquement au démarrage
- Les transactions MongoDB sont utilisées pour garantir l'atomicité des opérations complexes

