# Guide de Migration : Standardisation de la Gestion des Erreurs

## 📋 Vue d'ensemble

Ce guide explique comment migrer de `gqlerror.Errorf` vers `utils.AppError` pour standardiser la gestion des erreurs dans tout le codebase.

## ✅ Avantages de AppError

1. **Stack traces** : Capture automatique des stack traces pour le debugging
2. **Types d'erreurs** : Classification claire (Validation, NotFound, Database, etc.)
3. **Messages utilisateur** : Séparation entre messages techniques et messages utilisateur-friendly
4. **Location tracking** : Suivi automatique de l'emplacement de l'erreur
5. **Conversion GraphQL** : Conversion automatique vers `gqlerror.Error` pour GraphQL

## 🔄 Pattern de Migration

### Avant (gqlerror.Errorf)
```go
import "github.com/vektah/gqlparser/v2/gqlerror"

if err != nil {
    return nil, gqlerror.Errorf("Error message: %v", err)
}
```

### Après (AppError)
```go
import "rangoapp/utils"

if err != nil {
    return nil, utils.DatabaseErrorf("operation_name", "Error message: %v", err)
}
```

## 📝 Mapping des Types d'Erreurs

| Situation | Ancien (gqlerror) | Nouveau (AppError) |
|-----------|-------------------|-------------------|
| Validation | `gqlerror.Errorf("Invalid...")` | `utils.ValidationErrorf("Invalid...")` |
| Not Found | `gqlerror.Errorf("...not found")` | `utils.NotFoundErrorf("...not found")` |
| Database | `gqlerror.Errorf("Error...")` | `utils.DatabaseErrorf("op", "Error...")` |
| Unauthorized | `gqlerror.Errorf("Unauthorized")` | `utils.NewUnauthorizedError("...")` |
| Forbidden | `gqlerror.Errorf("Access denied")` | `utils.NewForbiddenError("...")` |

## 🛠️ Fonctions Helper Disponibles

### Erreurs de Validation
```go
utils.ValidationErrorf("Invalid payment type: %s", paymentType)
utils.NewValidationError("Invalid input")
```

### Erreurs Not Found
```go
utils.NotFoundErrorf("Product not found: %s", productID)
utils.NewNotFoundError("Product")
```

### Erreurs Database
```go
utils.DatabaseErrorf("create_sale", "Error creating sale: %v", err)
utils.NewDatabaseError("create_sale", err)
```

### Erreurs Unauthorized/Forbidden
```go
utils.NewUnauthorizedError("Invalid token")
utils.NewForbiddenError("Access denied")
```

### Wrapper d'erreurs
```go
utils.WrapError(err, "Failed to process")
```

## 📦 Fichiers Migrés

- ✅ `database/sale_db.go` - Complètement migré
- ⏳ `database/debt_db.go` - En cours
- ⏳ `database/provider_debt_db.go` - En cours
- ⏳ Autres fichiers `database/*_db.go` - À migrer

## 🔍 Exemples de Migration

### Exemple 1 : Erreur de validation
```go
// Avant
if !validPaymentTypes[paymentType] {
    return nil, gqlerror.Errorf("Invalid payment type: %s", paymentType)
}

// Après
if !validPaymentTypes[paymentType] {
    return nil, utils.ValidationErrorf("Invalid payment type: %s", paymentType)
}
```

### Exemple 2 : Erreur Not Found
```go
// Avant
if err == mongo.ErrNoDocuments {
    return nil, gqlerror.Errorf("Product not found")
}

// Après
if err == mongo.ErrNoDocuments {
    return nil, utils.NotFoundErrorf("Product not found")
}
```

### Exemple 3 : Erreur Database
```go
// Avant
_, err = collection.InsertOne(ctx, doc)
if err != nil {
    return nil, gqlerror.Errorf("Error creating document: %v", err)
}

// Après
_, err = collection.InsertOne(ctx, doc)
if err != nil {
    return nil, utils.DatabaseErrorf("create_document", "Error creating document: %v", err)
}
```

## ⚠️ Erreurs Ignorées

Les erreurs qui étaient ignorées dans `sale_db.go` ont été corrigées avec l'utilisation de transactions MongoDB. Toutes les opérations critiques sont maintenant dans une transaction, donc si une étape échoue, tout est rollback.

Pour les erreurs non-critiques (comme la création de transactions caisse après un paiement), on utilise maintenant `utils.LogError()` avec un message explicite :

```go
if err != nil {
    // Log error but don't fail the payment - the payment is already recorded
    utils.LogError(err, fmt.Sprintf("Failed to create caisse transaction for payment %s", paymentID))
}
```

## 🚀 Prochaines Étapes

1. Migrer tous les fichiers `database/*_db.go` vers AppError
2. Migrer les fichiers `graph/*.go` vers AppError
3. Migrer les fichiers `middlewares/*.go` vers AppError
4. Ajouter des tests pour vérifier que les stack traces fonctionnent correctement

## 📚 Références

- `utils/errors.go` - Définition de AppError et fonctions helper
- `database/sale_db.go` - Exemple complet de migration
