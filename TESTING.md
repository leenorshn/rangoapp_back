# Guide de Test - RangoApp Backend

Ce document décrit comment exécuter et ajouter des tests pour le système RangoApp Backend.

## Structure des Tests

Les tests sont organisés en plusieurs catégories:

### 1. Tests Unitaires (`*_test.go`)

Tests isolés sans dépendances externes:
- `utils/*_test.go` - Tests des utilitaires (JWT, password, errors, logger)
- `validators/*_test.go` - Tests des validateurs
- `middlewares/*_test.go` - Tests des middlewares
- `services/*_test.go` - Tests des services

**Exécution:**
```bash
make test-unit
# ou
go test ./utils/... ./validators/... ./middlewares/... ./services/... -v
```

### 2. Tests d'Intégration Database (`database/*_test.go`)

Tests avec connexion MongoDB réelle:
- `database/*_db_test.go` - Tests pour chaque module database

**Exécution:**
```bash
export TEST_MONGO_URI="mongodb://localhost:27017"
export TEST_MONGO_DB_NAME="rangoapp_test"
make test-integration
# ou
go test ./database/... -v
```

### 3. Tests GraphQL (`graph/*_test.go`)

Tests des resolvers GraphQL:
- `graph/auth_test.go` - Tests d'authentification
- `graph/mutations_test.go` - Tests des mutations
- `graph/queries_test.go` - Tests des queries

**Exécution:**
```bash
export TEST_MONGO_URI="mongodb://localhost:27017"
go test ./graph/... -v
```

### 4. Tests End-to-End (`e2e/*_test.go`)

Tests complets de workflows métier:
- `e2e/registration_workflow_test.go`
- `e2e/sale_workflow_test.go`
- `e2e/debt_workflow_test.go`
- etc.

**Exécution:**
```bash
export TEST_MONGO_URI="mongodb://localhost:27017"
make test-e2e
# ou
go test ./e2e/... -v
```

## Configuration

### Variables d'Environnement

Pour les tests d'intégration et end-to-end, configurez:

```bash
export TEST_MONGO_URI="mongodb://localhost:27017"
export TEST_MONGO_DB_NAME="rangoapp_test"  # Optionnel, défaut: rangoapp_test
```

### Fichier `.env.test`

Vous pouvez créer un fichier `.env.test` pour les tests:

```env
TEST_MONGO_URI=mongodb://localhost:27017
TEST_MONGO_DB_NAME=rangoapp_test
```

## Exécution des Tests

### Tous les tests
```bash
make test
```

### Tests unitaires uniquement
```bash
make test-unit
```

### Tests d'intégration
```bash
make test-integration
```

### Tests avec couverture
```bash
make test-coverage
```

### Tests spécifiques
```bash
go test ./utils/... -v
go test ./database/user_db_test.go -v
go test -run TestValidateEmail -v
```

## Ajout de Nouveaux Tests

### Structure d'un Test

```go
package database

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFunctionName(t *testing.T) {
    // Arrange
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // Act
    result, err := db.SomeFunction()
    
    // Assert
    require.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Helpers Disponibles

Dans `database/test_helpers.go`:
- `createTestCompany()` - Créer une company de test
- `createTestUser()` - Créer un utilisateur de test
- `createTestStore()` - Créer un store de test
- `createTestProduct()` - Créer un produit de test
- `createTestClient()` - Créer un client de test
- `createTestProvider()` - Créer un fournisseur de test

**Note:** `setupTestDB()` et `cleanupTestDB()` sont définis dans `database/subscription_plan_db_test.go`.

### Bonnes Pratiques

1. **Isolation**: Chaque test doit être indépendant
2. **Nettoyage**: Toujours nettoyer les données après chaque test
3. **Fixtures**: Utiliser les factories pour créer des données de test
4. **Noms**: Utiliser des noms descriptifs pour les tests
5. **Assertions**: Utiliser `require` pour les erreurs critiques, `assert` pour les validations

## Couverture de Code

Générer un rapport de couverture:

```bash
make test-coverage
```

Ouvrir le rapport:
```bash
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

Objectif: **Minimum 80% de couverture**

## CI/CD

Les tests sont exécutés automatiquement dans CI/CD. Voir `.github/workflows/test.yml` pour la configuration.

## Dépannage

### Tests qui échouent

1. Vérifier que MongoDB est accessible
2. Vérifier les variables d'environnement
3. Vérifier que la base de test est propre
4. Exécuter avec `-v` pour plus de détails

### Tests qui timeout

1. Vérifier la connexion MongoDB
2. Augmenter les timeouts si nécessaire
3. Vérifier que les tests nettoient correctement

### Erreurs de connexion

```bash
# Vérifier que MongoDB est en cours d'exécution
mongosh --eval "db.adminCommand('ping')"

# Vérifier la connexion de test
export TEST_MONGO_URI="mongodb://localhost:27017"
go test ./database/... -v -run TestGetExchangeRate
```

## Ressources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Testify Documentation](https://github.com/stretchr/testify)
- [MongoDB Go Driver](https://www.mongodb.com/docs/drivers/go/current/)





