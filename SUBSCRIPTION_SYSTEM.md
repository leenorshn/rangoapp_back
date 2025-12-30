# Système d'Abonnement et Blocage Automatique

## Vue d'ensemble

Ce document décrit le système d'abonnement et de blocage automatique des clients après la période d'essai pour l'application RangoApp SAAS.

## Architecture

### 1. Modèle de Données

Le modèle `Subscription` est défini dans `database/subscription_db.go` avec les champs suivants :
- **Plan** : "trial", "starter", "business", "enterprise"
- **Status** : "active", "expired", "cancelled", "suspended"
- **TrialStartDate** / **TrialEndDate** : Dates de début et fin de l'essai
- **SubscriptionStartDate** / **SubscriptionEndDate** : Dates pour les abonnements payants
- **MaxStores** / **MaxUsers** : Limites selon le plan

### 2. Plans Tarifaires

Les limites par plan sont définies dans `PlanLimits` :

- **Trial** : 14 jours, 1 store, 2 utilisateurs
- **Starter** : 1 store, 1 utilisateur
- **Business** : 3 stores, 5 utilisateurs
- **Enterprise** : Illimité

### 3. Création Automatique d'Essai

Lors de la création d'une entreprise (`CreateCompany`), un essai de 14 jours est automatiquement créé via `CreateTrialSubscription`.

### 4. Vérification d'Abonnement

#### Dans le Login
Le service d'authentification vérifie l'abonnement lors du login. Si l'essai est expiré, l'accès est bloqué avec le message :
> "Votre période d'essai a expiré. Veuillez vous abonner pour continuer à utiliser l'application."

#### Dans les Mutations Critiques
Les mutations suivantes vérifient l'abonnement avant d'autoriser l'action :
- `CreateProduct`
- `CreateSale`
- `CreateFacture`
- `CreateCaisseTransaction`
- `CreateClient`
- `CreateProvider`
- `CreateRapportStore`
- `CreateStore` (vérifie aussi les limites)
- `CreateUser` (vérifie aussi les limites)

### 5. Blocage Automatique

#### Tâche Cron
Une tâche cron s'exécute toutes les heures pour :
1. Trouver tous les essais expirés
2. Mettre à jour leur statut à "expired"
3. Bloquer l'accès à ces entreprises

La tâche est démarrée automatiquement au lancement du serveur via `services.StartCronJobs()`.

### 6. Middleware de Vérification

Le middleware `ValidateSubscriptionInContext` peut être utilisé dans les resolvers pour vérifier l'abonnement :
```go
if err := r.CheckSubscription(ctx); err != nil {
    return nil, err
}
```

## Utilisation

### Vérifier l'Abonnement d'une Entreprise

```graphql
query {
  subscription {
    id
    plan
    status
    daysRemaining
    isTrialExpired
    maxStores
    maxUsers
  }
}
```

### Vérifier le Statut de l'Abonnement

```graphql
query {
  checkSubscriptionStatus {
    isValid
    message
    subscription {
      plan
      status
      daysRemaining
    }
  }
}
```

### Créer un Abonnement Payant

```graphql
mutation {
  createSubscription(
    plan: "business"
    paymentMethod: "stripe"
    paymentId: "pay_xxx"
  ) {
    id
    plan
    status
  }
}
```

### Upgrader un Abonnement

```graphql
mutation {
  upgradeSubscription(
    plan: "enterprise"
    paymentMethod: "stripe"
    paymentId: "pay_xxx"
  ) {
    id
    plan
    maxStores
    maxUsers
  }
}
```

### Annuler un Abonnement

```graphql
mutation {
  cancelSubscription
}
```

## Messages d'Erreur

### Essai Expiré
- **Login** : "Votre période d'essai a expiré. Veuillez vous abonner pour continuer à utiliser l'application."
- **Mutations** : "Trial period expired. Please subscribe to continue."

### Abonnement Inactif
- "Subscription is not active. Please renew your subscription."

### Limites Atteintes
- **Stores** : "Store limit reached (X/Y). Please upgrade your plan."
- **Users** : "User limit reached (X/Y). Please upgrade your plan."

## Configuration

### Variables d'Environnement

Aucune variable d'environnement spécifique n'est requise pour le système d'abonnement. Les durées d'essai et les limites sont définies dans le code.

### Personnalisation

Pour modifier la durée de l'essai, éditez `PlanLimits["trial"].TrialDays` dans `database/subscription_db.go`.

Pour modifier les limites des plans, éditez les valeurs dans `PlanLimits`.

## Intégration de Paiement

Le système est prêt pour l'intégration avec des processeurs de paiement. Les champs `paymentMethod` et `paymentId` sont disponibles dans le modèle `Subscription`.

### Processeurs Recommandés

- **Stripe** : International
- **PayPal** : International
- **Mobile Money** : Orange Money, M-Pesa (Afrique)
- **Flutterwave** : Afrique
- **Paystack** : Afrique de l'Ouest

## Monitoring

Les logs suivants sont générés :
- Démarrage des tâches cron
- Vérification des essais expirés
- Blocage d'entreprises
- Erreurs de vérification d'abonnement

## Prochaines Étapes

1. **Intégration de Paiement** : Implémenter l'intégration avec un processeur de paiement
2. **Notifications** : Envoyer des emails de notification avant et après l'expiration
3. **Mode Read-Only** : Implémenter un mode lecture seule pendant 7 jours après expiration
4. **Tableau de Bord** : Créer une interface pour gérer les abonnements
5. **Métriques** : Ajouter des métriques sur les conversions d'essai vers abonnement

























