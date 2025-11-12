# Prompt Complet pour Créer un Serveur GraphQL avec Golang, gqlgen et MongoDB Atlas

## Contexte du Projet

Créer un serveur backend GraphQL en Golang utilisant gqlgen et MongoDB Atlas pour une application de gestion multi-points-de-vente (RangoApp). Le système permet à une entreprise (Company) de gérer plusieurs boutiques/magasins (Store), chaque boutique ayant ses propres produits, clients, factures, etc. Le serveur doit fournir toutes les fonctionnalités nécessaires pour gérer les opérations de l'application mobile Android.

**Architecture Multi-Boutiques**:
- Une **Company** (entreprise) peut avoir plusieurs **Stores** (boutiques/magasins)
- Chaque **Store** a ses propres produits, clients, factures, fournisseurs et rapports de stock
- Un utilisateur **Admin** peut accéder à toutes les boutiques de son entreprise
- Un utilisateur **User** (non-admin) peut être affecté à une seule boutique spécifique
- Lors de la création d'une Company, un utilisateur Admin et un premier Store sont créés automatiquement

## Architecture Technique

- **Langage**: Golang (Go 1.21+)
- **Framework GraphQL**: gqlgen (https://github.com/99designs/gqlgen)
- **Base de données**: MongoDB Atlas
- **Driver MongoDB**: go.mongodb.org/mongo-driver
- **Authentification**: JWT (JSON Web Tokens)
- **Validation**: Utiliser des validators pour les entrées
- **Structure**: Architecture propre avec séparation des couches (resolvers, services, repositories, models)

## Structure du Projet

```
rango-server/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── database/
│   │   └── mongodb.go
│   ├── models/
│   │   ├── user.go
│   │   ├── company.go
│   │   ├── store.go
│   │   ├── product.go
│   │   ├── client.go
│   │   ├── provider.go
│   │   ├── facture.go
│   │   └── rapport_store.go
│   ├── repository/
│   │   ├── user_repository.go
│   │   ├── company_repository.go
│   │   ├── store_repository.go
│   │   ├── product_repository.go
│   │   ├── client_repository.go
│   │   ├── provider_repository.go
│   │   ├── facture_repository.go
│   │   └── rapport_store_repository.go
│   ├── service/
│   │   ├── auth_service.go
│   │   ├── user_service.go
│   │   ├── company_service.go
│   │   ├── store_service.go
│   │   ├── product_service.go
│   │   ├── client_service.go
│   │   ├── provider_service.go
│   │   ├── facture_service.go
│   │   └── rapport_store_service.go
│   ├── middleware/
│   │   ├── auth.go
│   │   └── cors.go
│   └── utils/
│       ├── jwt.go
│       └── password.go
├── graph/
│   ├── schema.graphqls
│   ├── schema.resolvers.go
│   ├── model/
│   │   ├── models_gen.go
│   │   └── ...
│   └── generated.go
├── go.mod
├── go.sum
├── .env.example
└── README.md
```

## Modèles de Données

### 1. User (Utilisateur)
```graphql
type User {
  id: ID!
  uid: String!
  name: String!
  phone: String!
  email: String
  role: String! # Admin, User, etc.
  isBlocked: Boolean!
  companyId: String!
  storeIds: [String!]! # Liste des stores pour Admin, un seul store pour User
  assignedStoreId: String # Store assigné (pour User non-admin)
  createdAt: String!
  updatedAt: String!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `uid`: String (unique, identifiant utilisateur)
- `name`: String
- `phone`: String
- `email`: String (optionnel)
- `password`: String (hashé avec bcrypt)
- `role`: String (Admin, User, etc.)
- `isBlocked`: Boolean
- `companyId`: ObjectID (référence à Company)
- `storeIds`: Array of ObjectID (pour Admin: tous les stores de la company, pour User: un seul store)
- `assignedStoreId`: ObjectID (optionnel, store assigné pour User non-admin)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Règles**:
- Si `role = "Admin"`: peut accéder à tous les stores de sa company (`storeIds` contient tous les stores)
- Si `role = "User"`: ne peut accéder qu'au store dans `assignedStoreId` (un seul store)

### 2. Company (Entreprise)
```graphql
type Company {
  id: ID!
  name: String!
  address: String!
  phone: String!
  email: String
  description: String!
  type: String!
  logo: String
  rccm: String
  idNat: String
  idCommerce: String
  stores: [Store!]! # Liste des boutiques de l'entreprise
  createdAt: String!
  updatedAt: String!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `name`: String
- `address`: String
- `phone`: String
- `email`: String (optionnel)
- `description`: String
- `type`: String
- `logo`: String (optionnel, URL ou base64)
- `rccm`: String (optionnel)
- `idNat`: String (optionnel)
- `idCommerce`: String (optionnel)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Note**: Les stores sont liés à la company via le champ `companyId` dans le modèle Store.

### 3. Store (Boutique/Magasin)
```graphql
type Store {
  id: ID!
  name: String!
  address: String!
  phone: String!
  companyId: String!
  company: Company!
  createdAt: String!
  updatedAt: String!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `name`: String
- `address`: String
- `phone`: String
- `companyId`: ObjectID (référence à Company)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Règles**:
- Chaque Company doit avoir au moins un Store (créé automatiquement lors de l'inscription)
- Un Store appartient à une seule Company
- Toutes les données opérationnelles (products, clients, factures, etc.) sont liées à un Store spécifique

### 4. Product (Produit)
```graphql
type Product {
  id: ID!
  name: String!
  mark: String!
  priceVente: Float!
  priceAchat: Float!
  stock: Float!
  storeId: String!
  store: Store!
  createdAt: String!
  updatedAt: String!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `name`: String
- `mark`: String
- `priceVente`: Float64
- `priceAchat`: Float64
- `stock`: Float64
- `storeId`: ObjectID (référence à Store, pas Company)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Règles**:
- Chaque produit appartient à un Store spécifique
- Le stock est géré au niveau du Store
- Un même produit peut exister dans plusieurs stores avec des stocks différents

### 5. Client
```graphql
type Client {
  id: ID!
  name: String!
  phone: String!
  storeId: String!
  store: Store!
  createdAt: String!
  updatedAt: String!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `name`: String
- `phone`: String
- `storeId`: ObjectID (référence à Store)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Règles**:
- Chaque client appartient à un Store spécifique
- Un même client peut exister dans plusieurs stores (numéro de téléphone peut être identique)

### 6. Provider (Fournisseur)
```graphql
type Provider {
  id: ID!
  name: String!
  phone: String!
  address: String!
  storeId: String!
  store: Store!
  createdAt: String!
  updatedAt: String!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `name`: String
- `phone`: String
- `address`: String
- `storeId`: ObjectID (référence à Store)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Règles**:
- Chaque fournisseur appartient à un Store spécifique

### 7. Facture (Facture/Vente)
```graphql
type Facture {
  id: ID!
  factureNumber: String!
  products: [FactureProduct!]!
  quantity: Int!
  date: String!
  price: Float!
  currency: String!
  client: Client!
  storeId: String!
  store: Store!
  createdAt: String!
  updatedAt: String!
}

type FactureProduct {
  productId: String!
  product: Product!
  quantity: Int!
  price: Float!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `factureNumber`: String (unique par store)
- `products`: Array of objects avec `productId`, `quantity`, `price`
- `quantity`: Int (quantité totale)
- `date`: DateTime
- `price`: Float64
- `currency`: String
- `clientId`: ObjectID (référence à Client)
- `storeId`: ObjectID (référence à Store)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Règles**:
- Chaque facture appartient à un Store spécifique
- Le numéro de facture est unique par Store (pas par Company)
- Les produits dans la facture doivent appartenir au même Store

### 8. RapportStore (Rapport de Stock)
```graphql
type RapportStore {
  id: ID!
  type: String! # "entree" ou "sortie"
  product: Product!
  quantity: Float!
  date: String!
  storeId: String!
  store: Store!
  createdAt: String!
  updatedAt: String!
}
```

**Champs MongoDB**:
- `_id`: ObjectID
- `type`: String ("entree" ou "sortie")
- `productId`: ObjectID (référence à Product)
- `quantity`: Float64
- `date`: DateTime
- `storeId`: ObjectID (référence à Store)
- `createdAt`: DateTime
- `updatedAt`: DateTime

**Règles**:
- Chaque rapport appartient à un Store spécifique
- Le produit dans le rapport doit appartenir au même Store

## Schéma GraphQL Complet

### Queries
```graphql
type Query {
  # Auth
  me: User!
  
  # Users
  users: [User!]!
  user(id: ID!): User
  
  # Company
  company: Company!
  
  # Stores
  stores: [Store!]!
  store(id: ID!): Store
  
  # Products
  products(storeId: String): [Product!]! # Si storeId non fourni, retourne les produits des stores accessibles
  product(id: ID!): Product
  
  # Clients
  clients(storeId: String): [Client!]! # Si storeId non fourni, retourne les clients des stores accessibles
  client(id: ID!): Client
  
  # Providers
  providers(storeId: String): [Provider!]! # Si storeId non fourni, retourne les fournisseurs des stores accessibles
  provider(id: ID!): Provider
  
  # Factures
  factures(storeId: String): [Facture!]! # Si storeId non fourni, retourne les factures des stores accessibles
  facture(id: ID!): Facture
  
  # RapportStore
  rapportStore(storeId: String): [RapportStore!]! # Si storeId non fourni, retourne les rapports des stores accessibles
  rapportStoreById(id: ID!): RapportStore
}
```

### Mutations
```graphql
type Mutation {
  # Auth
  login(phone: String!, password: String!): AuthResponse!
  register(input: RegisterInput!): AuthResponse!
  logout: Boolean!
  
  # Users
  createUser(input: CreateUserInput!): User!
  updateUser(id: ID!, input: UpdateUserInput!): User!
  deleteUser(id: ID!): Boolean!
  blockUser(id: ID!): User!
  unblockUser(id: ID!): User!
  assignUserToStore(userId: ID!, storeId: ID!): User! # Assigner un utilisateur User à un store
  
  # Company
  updateCompany(input: UpdateCompanyInput!): Company!
  
  # Stores
  createStore(input: CreateStoreInput!): Store!
  updateStore(id: ID!, input: UpdateStoreInput!): Store!
  deleteStore(id: ID!): Boolean!
  
  # Products
  createProduct(input: CreateProductInput!): Product!
  updateProduct(id: ID!, input: UpdateProductInput!): Product!
  deleteProduct(id: ID!): Boolean!
  
  # Clients
  createClient(input: CreateClientInput!): Client!
  updateClient(id: ID!, input: UpdateClientInput!): Client!
  deleteClient(id: ID!): Boolean!
  
  # Providers
  createProvider(input: CreateProviderInput!): Provider!
  updateProvider(id: ID!, input: UpdateProviderInput!): Provider!
  deleteProvider(id: ID!): Boolean!
  
  # Factures
  createFacture(input: CreateFactureInput!): Facture!
  updateFacture(id: ID!, input: UpdateFactureInput!): Facture!
  deleteFacture(id: ID!): Boolean!
  
  # RapportStore
  createRapportStore(input: CreateRapportStoreInput!): RapportStore!
  deleteRapportStore(id: ID!): Boolean!
}
```

### Input Types
```graphql
input RegisterInput {
  # Informations utilisateur Admin
  email: String!
  password: String!
  name: String!
  phone: String!
  
  # Informations Company
  companyName: String!
  companyAddress: String!
  companyPhone: String!
  companyDescription: String!
  companyType: String!
  companyEmail: String
  companyLogo: String
  companyRccm: String
  companyIdNat: String
  companyIdCommerce: String
  
  # Informations du premier Store (boutique)
  storeName: String!
  storeAddress: String!
  storePhone: String!
}

input CreateUserInput {
  name: String!
  phone: String!
  email: String
  password: String!
  role: String! # "Admin" ou "User"
  storeId: String # Optionnel: si role="User", assigner directement à un store
}

input UpdateUserInput {
  name: String
  phone: String
  email: String
  role: String
  storeId: String # Pour changer l'assignation de store (si role="User")
}

input UpdateCompanyInput {
  name: String
  address: String
  phone: String
  email: String
  description: String
  type: String
  logo: String
  rccm: String
  idNat: String
  idCommerce: String
}

input CreateStoreInput {
  name: String!
  address: String!
  phone: String!
}

input UpdateStoreInput {
  name: String
  address: String
  phone: String
}

input CreateProductInput {
  name: String!
  mark: String!
  priceVente: Float!
  priceAchat: Float!
  stock: Float!
  storeId: String! # Store auquel appartient le produit
}

input UpdateProductInput {
  name: String
  mark: String
  priceVente: Float
  priceAchat: Float
  stock: Float
}

input CreateClientInput {
  name: String!
  phone: String!
  storeId: String! # Store auquel appartient le client
}

input UpdateClientInput {
  name: String
  phone: String
}

input CreateProviderInput {
  name: String!
  phone: String!
  address: String!
  storeId: String! # Store auquel appartient le fournisseur
}

input UpdateProviderInput {
  name: String
  phone: String
  address: String
}

input FactureProductInput {
  productId: String!
  quantity: Int!
  price: Float!
}

input CreateFactureInput {
  products: [FactureProductInput!]!
  clientId: String!
  storeId: String! # Store pour lequel la facture est créée
  quantity: Int!
  price: Float!
  currency: String!
  date: String!
}

input UpdateFactureInput {
  products: [FactureProductInput!]
  clientId: String
  quantity: Int
  price: Float
  currency: String
  date: String
}

input CreateRapportStoreInput {
  productId: String!
  storeId: String! # Store pour lequel le rapport est créé
  quantity: Float!
  type: String! # "entree" ou "sortie"
  date: String!
}

type AuthResponse {
  token: String!
  user: User!
}
```

## Fonctionnalités Requises

### 1. Authentification
- **Login**: Authentification par email/phone et mot de passe
- **Register**: Création de compte avec création automatique :
  - Création d'une Company (entreprise)
  - Création d'un premier utilisateur Admin (celui qui s'inscrit)
  - Création d'un premier Store (boutique) avec les informations fournies
  - Association automatique de l'Admin à tous les stores de la company (via storeIds)
- **JWT**: Génération de tokens JWT pour l'authentification
- **Middleware Auth**: Protection des routes nécessitant une authentification
- **Hashage de mots de passe**: Utiliser bcrypt pour sécuriser les mots de passe

### 2. Gestion des Utilisateurs
- CRUD complet pour les utilisateurs
- Gestion des rôles (Admin, User)
- Blocage/Déblocage d'utilisateurs
- Association des utilisateurs à une entreprise
- **Affectation des utilisateurs aux stores**:
  - Les Admin ont accès à tous les stores de leur company (storeIds contient tous les stores)
  - Les User peuvent être affectés à un seul store (assignedStoreId)
  - Mutation `assignUserToStore` pour affecter un User à un store
  - Lors de la création d'un User avec role="User", optionnellement assigner directement à un store

### 3. Gestion de l'Entreprise
- **Création**: Automatique lors de l'inscription (register)
- Mise à jour de l'entreprise (seulement pour Admin)
- Récupération de l'entreprise de l'utilisateur connecté
- Validation des champs optionnels (RCCM, ID Nat, etc.)
- Récupération de la liste des stores de l'entreprise

### 4. Gestion des Stores (Boutiques)
- CRUD complet pour les stores (seulement pour Admin)
- Création automatique du premier store lors de l'inscription
- Chaque store a ses propres produits, clients, factures, fournisseurs
- Validation que le store appartient à la company de l'utilisateur
- Un Admin peut créer plusieurs stores pour sa company

### 5. Gestion des Produits
- CRUD complet pour les produits
- Gestion du stock au niveau du Store
- Association des produits à un Store spécifique (pas à la Company)
- Validation des prix (prix de vente >= prix d'achat)
- **Isolation par Store**: Un utilisateur ne peut voir/modifier que les produits des stores auxquels il a accès
- Un même produit peut exister dans plusieurs stores avec des stocks différents

### 6. Gestion des Clients
- CRUD complet pour les clients
- Association des clients à un Store spécifique
- Validation du numéro de téléphone
- **Isolation par Store**: Un utilisateur ne peut voir/modifier que les clients des stores auxquels il a accès

### 7. Gestion des Fournisseurs
- CRUD complet pour les fournisseurs
- Association des fournisseurs à un Store spécifique
- **Isolation par Store**: Un utilisateur ne peut voir/modifier que les fournisseurs des stores auxquels il a accès

### 8. Gestion des Factures/Ventes
- Création de factures avec plusieurs produits
- Génération automatique du numéro de facture (unique par Store, pas par Company)
- Mise à jour automatique du stock lors de la création d'une facture
- Calcul automatique du prix total
- Association des factures à un Store et un client
- Validation que tous les produits appartiennent au même Store
- **Isolation par Store**: Un utilisateur ne peut voir/modifier que les factures des stores auxquels il a accès

### 9. Gestion des Rapports de Stock
- Création de rapports d'entrée/sortie de stock
- Mise à jour automatique du stock lors de la création d'un rapport
- Filtrage par type (entrée/sortie)
- Association des rapports à un Store spécifique
- Validation que le produit appartient au Store du rapport
- **Isolation par Store**: Un utilisateur ne peut voir/modifier que les rapports des stores auxquels il a accès

## Règles Métier

1. **Isolation Multi-Niveaux**:
   - **Niveau 1 - Company**: Toutes les données appartiennent à une Company
   - **Niveau 2 - Store**: Les données opérationnelles (products, clients, factures, etc.) appartiennent à un Store spécifique
   - **Niveau 3 - User Access**:
     - Un **Admin** peut accéder à tous les stores de sa Company
     - Un **User** ne peut accéder qu'au store auquel il est assigné (assignedStoreId)
   - Vérifier à chaque requête que l'utilisateur a accès au Store concerné

2. **Création Initiale (Register)**:
   - Lors de l'inscription, créer dans l'ordre :
     1. Company avec les informations fournies
     2. Premier Store avec les informations fournies (storeName, storeAddress, storePhone)
     3. Utilisateur Admin avec les informations fournies
     4. Associer l'Admin à la Company et ajouter le Store dans storeIds de l'Admin
   - Utiliser une transaction MongoDB pour garantir l'atomicité

3. **Gestion des Stores**:
   - Seuls les Admin peuvent créer/modifier/supprimer des stores
   - Lors de la création d'un nouveau store, mettre à jour automatiquement les storeIds de tous les Admin de la Company
   - Un Store ne peut pas être supprimé s'il contient encore des données (produits, factures, etc.)

4. **Affectation des Utilisateurs**:
   - Seuls les Admin peuvent créer des utilisateurs
   - Lors de la création d'un User avec role="User", optionnellement assigner à un store via storeId
   - Un User ne peut être assigné qu'à un seul store à la fois
   - Utiliser la mutation `assignUserToStore` pour changer l'assignation
   - Lorsqu'un Admin crée un nouveau store, tous les Admin de la Company y ont automatiquement accès

5. **Gestion du Stock**:
   - Le stock est géré au niveau du Store (chaque Store a son propre stock pour chaque produit)
   - Lors de la création d'une facture, vérifier que le stock est suffisant dans le Store concerné
   - Déduire automatiquement le stock lors de la création d'une facture
   - Mettre à jour le stock lors de la création d'un rapport d'entrée/sortie
   - Utiliser des transactions MongoDB pour garantir la cohérence

6. **Numérotation des Factures**:
   - Générer automatiquement un numéro de facture unique par Store (pas par Company)
   - Format suggéré: `FACT-{STORE_ID}-{YYYY}-{NUMERO}` ou `FACT-{NUMERO}` (unique dans le Store)
   - Utiliser un compteur séquentiel par Store

7. **Validation**:
   - Valider tous les champs requis
   - Valider les formats (email, téléphone, etc.)
   - Valider les montants (positifs, prix de vente >= prix d'achat)
   - Valider que les produits dans une facture appartiennent tous au même Store
   - Valider que le client d'une facture appartient au même Store
   - Valider que l'utilisateur a accès au Store concerné avant toute opération

8. **Sécurité**:
   - Tous les endpoints (sauf login/register) doivent être protégés par JWT
   - Vérifier que l'utilisateur appartient à la Company avant toute opération
   - Vérifier que l'utilisateur a accès au Store concerné (Admin: tous les stores, User: seulement assignedStoreId)
   - Ne pas exposer les mots de passe dans les réponses
   - Les Admin peuvent gérer tous les stores de leur Company
   - Les User ne peuvent gérer que leur store assigné

## Configuration

### Variables d'Environnement (.env)
```
# Server
PORT=8080
ENV=development

# MongoDB Atlas
MONGODB_URI=mongodb+srv://username:password@cluster.mongodb.net/rangoapp?retryWrites=true&w=majority
MONGODB_DB_NAME=rangoapp

# JWT
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_EXPIRATION=24h

# CORS
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080
```

## Dépendances Go Requises

```go
require (
    github.com/99designs/gqlgen v0.17.40
    github.com/vektah/gqlparser/v2 v2.5.10
    go.mongodb.org/mongo-driver v1.13.0
    github.com/golang-jwt/jwt/v5 v5.2.0
    golang.org/x/crypto v0.17.0
    github.com/joho/godotenv v1.5.1
    github.com/rs/cors v1.10.1
)
```

## Instructions d'Implémentation

1. **Initialiser le projet**:
   - Créer la structure de dossiers
   - Initialiser `go.mod`
   - Configurer gqlgen avec `gqlgen init`

2. **Configuration MongoDB**:
   - Créer la connexion à MongoDB Atlas
   - Implémenter les fonctions de connexion/déconnexion
   - Créer les index nécessaires :
     - `users`: index unique sur `uid`, index sur `companyId`, index sur `storeIds`
     - `stores`: index sur `companyId`
     - `products`: index sur `storeId`
     - `clients`: index sur `storeId`
     - `providers`: index sur `storeId`
     - `factures`: index unique composé sur `(storeId, factureNumber)`, index sur `storeId`
     - `rapportStore`: index sur `storeId`, index sur `productId`

3. **Modèles**:
   - Créer les structs Go correspondant aux modèles MongoDB
   - Ajouter les tags BSON appropriés
   - Implémenter les méthodes de conversion vers/depuis GraphQL

4. **Repositories**:
   - Implémenter toutes les opérations CRUD pour chaque entité
   - Utiliser le contexte pour passer le companyId et storeId
   - Filtrer les résultats selon les stores accessibles par l'utilisateur
   - Pour les Admin: retourner les données de tous les stores de leur Company
   - Pour les User: retourner uniquement les données de leur assignedStoreId
   - Gérer les erreurs MongoDB appropriées

5. **Services**:
   - Implémenter la logique métier dans les services
   - Gérer la validation des données
   - Gérer les transactions MongoDB si nécessaire

6. **Resolvers GraphQL**:
   - Implémenter tous les resolvers pour Query et Mutation
   - Utiliser le middleware d'authentification pour extraire l'utilisateur
   - Appeler les services appropriés

7. **Authentification**:
   - Implémenter le service d'authentification
   - Créer le middleware JWT
   - Gérer la génération et validation des tokens
   - Dans le token JWT, inclure: userId, companyId, role, storeIds (ou assignedStoreId)
   - Dans le contexte GraphQL, stocker l'utilisateur avec ses informations d'accès aux stores

8. **Tests**:
   - Créer des tests unitaires pour les services
   - Créer des tests d'intégration pour les resolvers
   - Tester l'authentification et l'autorisation

## Points d'Attention

1. **Performance**: Utiliser des index MongoDB appropriés pour les requêtes fréquentes
2. **Sécurité**: Ne jamais exposer les mots de passe, toujours les hasher
3. **Validation**: Valider toutes les entrées utilisateur
4. **Gestion d'erreurs**: Retourner des erreurs GraphQL appropriées
5. **Transactions**: Utiliser des transactions MongoDB pour les opérations complexes (ex: création de facture + mise à jour du stock)
6. **Pagination**: Considérer l'ajout de pagination pour les listes (products, factures, etc.)

## Exemple de Requête GraphQL

```graphql
# Inscription - Crée automatiquement Company, Admin User et premier Store
mutation Register {
  register(input: {
    email: "admin@example.com"
    password: "password123"
    name: "John Doe"
    phone: "+1234567890"
    companyName: "Ma Boutique SARL"
    companyAddress: "123 Rue Example"
    companyPhone: "+1234567890"
    companyDescription: "Description de l'entreprise"
    companyType: "Retail"
    storeName: "Boutique Centrale"
    storeAddress: "123 Rue Example"
    storePhone: "+1234567890"
  }) {
    token
    user {
      id
      name
      email
      role
      companyId
      storeIds
    }
  }
}

# Connexion
mutation Login {
  login(phone: "+1234567890", password: "password123") {
    token
    user {
      id
      name
      email
      role
      companyId
      storeIds
      assignedStoreId
    }
  }
}

# Récupérer les stores de l'entreprise
query GetStores {
  stores {
    id
    name
    address
    phone
  }
}

# Récupérer les produits (tous les stores si Admin, ou store spécifique)
query GetProducts {
  products(storeId: "store123") {
    id
    name
    mark
    priceVente
    priceAchat
    stock
    store {
      id
      name
    }
  }
}

# Créer un nouveau store (Admin seulement)
mutation CreateStore {
  createStore(input: {
    name: "Boutique Succursale"
    address: "456 Autre Rue"
    phone: "+0987654321"
  }) {
    id
    name
    address
    phone
  }
}

# Créer un utilisateur et l'assigner à un store
mutation CreateUser {
  createUser(input: {
    name: "Jane User"
    phone: "+1111111111"
    email: "jane@example.com"
    password: "password123"
    role: "User"
    storeId: "store123"
  }) {
    id
    name
    role
    assignedStoreId
  }
}

# Assigner un utilisateur à un store
mutation AssignUserToStore {
  assignUserToStore(userId: "user123", storeId: "store456") {
    id
    name
    assignedStoreId
  }
}

# Créer une facture
mutation CreateFacture {
  createFacture(input: {
    storeId: "store123"
    products: [
      { productId: "product1", quantity: 2, price: 100.0 }
    ]
    clientId: "client1"
    quantity: 2
    price: 200.0
    currency: "USD"
    date: "2024-01-15T10:00:00Z"
  }) {
    id
    factureNumber
    price
    store {
      name
    }
    client {
      name
    }
  }
}
```

## Livrables Attendus

1. Code source complet du serveur
2. Fichier `schema.graphqls` complet
3. Configuration MongoDB avec index
4. Documentation API (peut utiliser GraphQL Playground)
5. Fichier `.env.example`
6. README avec instructions d'installation et de déploiement
7. Tests unitaires et d'intégration

---

**Note**: Ce prompt est basé sur l'analyse de l'application Android RangoApp. Toutes les fonctionnalités doivent être implémentées en respectant l'architecture et les modèles de données décrits ci-dessus.

