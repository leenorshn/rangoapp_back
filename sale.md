Modifications effectuées
1. Validateur de date (validators/validators.go)
ValidateDate accepte maintenant :
RFC3339 : 2024-01-01T00:00:00Z
Format HTML date input : 2024-01-01
2. Resolver CreateSale (graph/schema.resolvers.go)
Parsing de la date mis à jour pour accepter les deux formats
Si le format est YYYY-MM-DD, la date est convertie en début de journée (00:00:00) dans le fuseau horaire local
Formats acceptés
Le backend accepte maintenant :
Format RFC3339 complet : 2024-01-01T00:00:00Z (ou avec timezone)
Format date simple : 2024-01-01 (depuis un champ HTML <input type="date">)
Le frontend peut envoyer la date directement depuis le champ HTML <input type="date"> sans conversion. Le backend gère les deux formats automatiquement.
Note : Les autres mutations (CreateFacture, UpdateFacture, etc.) utilisent encore uniquement RFC3339. Si nécessaire, je peux les mettre à jour de la même manière.
quel format de date nous devons applique au front web pour satisfaire la creation de vente , pour le champ date

Vérification du format de date accepté par le backend pour la création de vente :
Format de date pour le champ date dans createSale
Le backend accepte deux formats pour le champ date dans la mutation createSale :
Format recommandé (le plus simple)
Format : YYYY-MM-DD (format HTML date input)
Exemple : "2024-01-15"
C'est le format natif d'un champ HTML <input type="date">. Vous pouvez l'utiliser directement sans conversion.
Format alternatif
Format RFC3339 : YYYY-MM-DDTHH:mm:ssZ
Exemple : "2024-01-15T00:00:00Z"
Recommandation pour le frontend web
Utilisez le format YYYY-MM-DD :
// Exemple avec un champ HTML date inputconst dateInput = document.querySelector('input[type="date"]');const dateValue = dateInput.value; // Format: "2024-01-15"// Envoyez directement cette valeur dans la mutation GraphQLconst mutation = {  date: dateValue  // "2024-01-15"};
Ou avec React/Next.js :
// Dans votre formulaireconst [date, setDate] = useState('');// Dans le handler de soumissionconst handleSubmit = async () => {  await createSale({    variables: {      input: {        // ... autres champs        date: date, // Format: "2024-01-15" (directement depuis <input type="date">)      }    }  });};
Notes importantes
Le champ date est optionnel : si non fourni, le backend utilise la date/heure actuelle.
Format simple : YYYY-MM-DD est accepté et converti automatiquement en début de journée (00:00:00) dans le fuseau horaire local du serveur.
Pas de conversion nécessaire : vous pouvez envoyer directement la valeur du champ <input type="date">.
En résumé : utilisez le format YYYY-MM-DD directement depuis votre champ HTML date input, sans conversion.