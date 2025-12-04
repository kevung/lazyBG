# Test de la Fonctionnalité d'Édition de Coups

## Cas de Test

### Test 1 : Édition Simple d'un Coup Valide
**Objectif** : Vérifier qu'un coup peut être modifié sans créer d'incohérence

1. Ouvrir un match existant
2. Sélectionner un coup (par exemple : coup 1, joueur 1)
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Modifier les dés : `54` → `65`
5. Modifier le coup : `24/20 13/8` → `24/18 13/8`
6. Appuyer sur `Entrée` pour valider
7. **Résultat attendu** : Le coup est modifié, aucune incohérence détectée

### Test 2 : Édition Créant une Incohérence
**Objectif** : Vérifier la détection d'incohérence

1. Ouvrir un match existant
2. Sélectionner un coup (par exemple : coup 1, joueur 1)
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Modifier le coup de manière invalide : `24/20 13/8` → `bar/20`
   (alors qu'aucun pion n'est sur la barre)
5. Appuyer sur `Entrée` pour valider
6. **Résultat attendu** :
   - Le coup est marqué en rouge dans la table
   - Les coups suivants sont aussi marqués en rouge
   - Un message d'erreur apparaît dans la barre d'état
   - Survoler le coup montre la raison de l'erreur

### Test 3 : Annulation d'une Édition
**Objectif** : Vérifier qu'on peut annuler une édition

1. Ouvrir un match existant
2. Sélectionner un coup
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Modifier les dés et le coup
5. Appuyer sur `Esc` (ou `Tab` à nouveau)
6. **Résultat attendu** :
   - L'édition est annulée
   - Les valeurs originales sont conservées
   - Le mode revient à NORMAL

### Test 4 : Édition Successive de Plusieurs Coups
**Objectif** : Vérifier qu'on peut éditer plusieurs coups d'affilée

1. Ouvrir un match existant
2. Sélectionner le coup 1, joueur 1
3. Appuyer sur `Tab`, modifier, valider avec `Entrée`
4. Naviguer vers le coup 2 avec les flèches
5. Appuyer sur `Tab`, modifier, valider avec `Entrée`
6. **Résultat attendu** : Les deux coups sont modifiés correctement

### Test 5 : Validation des Dés
**Objectif** : Vérifier que seuls des dés valides sont acceptés

1. Ouvrir un match existant
2. Sélectionner un coup
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Essayer de saisir des dés invalides :
   - `78` (hors limite)
   - `0` (invalide)
   - `abc` (non numérique)
5. **Résultat attendu** : 
   - Seuls les chiffres 1-6 sont acceptés
   - Un message d'erreur apparaît si on essaie de valider avec des dés invalides

### Test 6 : Navigation au Clavier en Mode EDIT
**Objectif** : Vérifier les raccourcis clavier

1. Ouvrir un match existant
2. Sélectionner un coup
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Tester les touches :
   - `Entrée` dans le champ dés → passe au champ coup
   - `Entrée` dans le champ coup → valide l'édition
   - `Esc` → annule l'édition
   - `Tab` → annule l'édition
5. **Résultat attendu** : Tous les raccourcis fonctionnent comme prévu

## Scénarios d'Erreur à Tester

### Scénario 1 : Coup Impossible
- Éditer un coup pour déplacer un pion qui n'existe pas
- **Attendu** : Incohérence détectée, coup en rouge

### Scénario 2 : Nombre de Pions Incorrect
- Éditer un coup qui fait disparaître ou apparaître des pions
- **Attendu** : Validation échoue, nombre de pions ≠ 15

### Scénario 3 : Coups en Cascade
- Éditer un coup qui invalide tous les coups suivants
- **Attendu** : Tous les coups suivants sont en rouge avec message "Previous move caused inconsistency"

## Notes de Test

- Les coups spéciaux `Cannot Move` et `????` ne causent pas d'incohérence
- Le cache des positions est invalidé automatiquement après modification
- La validation est asynchrone mais rapide
- L'interface d'édition se centre automatiquement à l'écran
