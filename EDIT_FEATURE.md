# Fonctionnalité d'Édition de Coups - LazyBG

## Résumé des Modifications

Cette mise à jour ajoute la capacité d'éditer des coups existants dans LazyBG, avec validation automatique, détection d'incohérences, gestion des frappes et validation de la règle de la barre.

## Nouvelles Fonctionnalités

### 1. Mode EDIT
- **Activation** : Appuyez sur `Tab` pour basculer entre le mode NORMAL et le mode EDIT
- **Indication** : Le mode actuel est affiché dans la barre d'état
- Fonctionne uniquement lorsqu'un coup est sélectionné

### 2. Interface d'Édition
- **Panel modal** qui s'ouvre en mode EDIT
- **Champ Dés** : 
  - Saisir 2 chiffres entre 1 et 6
  - Auto-avance vers le champ suivant après saisie valide
  - Exemple : `54` pour un 5 et un 4
- **Champ Coup** :
  - Notation backgammon standard
  - Exemples : `24/20 13/8`, `bar/21`, `6/off 4/off`

### 3. Validation et Annulation
- **Entrée** : Valide et enregistre les modifications
- **Esc ou Tab** : Annule l'édition et restaure les valeurs originales
- **Boutons** : Interface alternative pour valider/annuler

### 4. Détection d'Incohérences
- **Validation automatique** après chaque modification
- **Recalcul des positions** pour tous les coups suivants
- **Surlignage en rouge** des coups incohérents dans la table des coups
- **Info-bulle** : Survolez un coup incohérent pour voir la raison de l'erreur

### 5. Gestion Automatique des Frappes
- **Détection automatique** : Lorsqu'un coup frappe un pion adverse
- **Ajout de l'étoile (*)** : Le coup adverse suivant est automatiquement marqué avec *
- **Message de confirmation** : La barre d'état indique qu'une frappe a été détectée

### 6. Validation de la Règle de la Barre
- **Règle stricte** : Si un joueur a des pions à la barre, il DOIT les faire entrer avant de jouer ailleurs
- **Validation automatique** : Les coups qui violent cette règle sont marqués incohérents
- **Message d'erreur** : "Must enter from bar before moving other checkers"

### 7. Règles de Cohérence
Un coup est marqué comme incohérent si :
- Le nombre de pions est incorrect (≠ 15 par joueur)
- Le coup ne peut pas être appliqué à la position précédente
- Un pion à la barre n'est pas entré en premier
- Un coup précédent a causé une incohérence

## Fichiers Modifiés

### Nouveaux Fichiers
- `frontend/src/components/EditMovePanel.svelte` : Interface d'édition de coups

### Fichiers Modifiés
- `frontend/src/App.svelte` : Intégration du composant EditMovePanel
- `frontend/src/stores/transcriptionStore.js` : 
  - Ajout du store `moveInconsistenciesStore`
  - Nouvelle fonction `validateGameInconsistencies()`
  - Export du module `get` de Svelte
- `frontend/src/utils/positionCalculator.js` :
  - Modification de `applyMove()` pour retourner `{ position, hasHit }`
  - Modification de `applyMoveSegment()` pour détecter les frappes
  - Nouvelle fonction `validateGamePositions()` avec validation de la règle de la barre
  - Nouvelle fonction `validateBarFirstRule()` pour vérifier les coups depuis la barre
  - Nouvelles fonctions `addHitMarker()` et `removeHitMarker()` pour gérer l'étoile
  - Détection améliorée des erreurs de position
- `frontend/src/components/MovesTable.svelte` :
  - Import du store `moveInconsistenciesStore`
  - Affichage des incohérences avec style rouge
  - Info-bulles pour expliquer les erreurs
  - Nouvelles classes CSS pour les coups incohérents

## Utilisation

### Éditer un Coup
1. Sélectionnez un coup dans la table des coups (clic ou navigation clavier)
2. Appuyez sur `Tab` pour entrer en mode EDIT
3. Le panel d'édition s'ouvre avec les valeurs actuelles
4. Modifiez les dés (2 chiffres 1-6)
5. Modifiez le coup (notation standard)
6. Appuyez sur `Entrée` pour valider ou `Esc`/`Tab` pour annuler

### Interpréter les Incohérences
- **Fond rouge** : Le coup a un problème de cohérence
- **Survolez** le coup pour voir le message d'erreur
- Les coups suivants sont aussi marqués s'ils sont affectés
- Corrigez le premier coup incohérent pour résoudre le problème

## Workflow de Validation

1. **Modification d'un coup** → Invalidation du cache de positions
2. **Validation des positions** → Calcul de toutes les positions suivantes
3. **Détection d'erreurs** → Identification des coups problématiques
4. **Affichage visuel** → Surlignage en rouge dans la table
5. **Information** → Message dans la barre d'état

## Notes Techniques

- Les positions sont recalculées de manière asynchrone après modification
- Le cache de positions est invalidé à partir du coup modifié
- La validation s'arrête au premier coup incohérent pour éviter les erreurs en cascade
- Les coups spéciaux (`Cannot Move`, `????`) sont ignorés lors de la validation
- La fonction `applyMove()` retourne maintenant `{ position, hasHit }` au lieu de juste `position`
- Les marqueurs de frappe (*) sont automatiquement retirés avant validation et ajoutés après détection
- La règle de la barre est validée pour chaque coup en vérifiant que tous les mouvements depuis la barre sont prioritaires
