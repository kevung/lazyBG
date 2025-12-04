# Auto-Correction des marqueurs de frappe (*) - Implémentation

## Problème résolu

Quand l'utilisateur édite un coup (par exemple move 6 player 2 de "43 15/8" en "42 15/9"), les coups suivants détectaient les incohérences mais ne corrigeaient pas automatiquement la notation avec les étoiles (*).

### Exemple du problème
- Édition du move 6 player 2 : `43 15/8` → `42 15/9`
- Move 12 player 2 affichait : `51 Bar/24* 21/16` (une seule frappe)
- Move 12 player 2 devrait être : `51 Bar/24* 21/16*` (deux frappes)

## Solution implémentée

### 1. Fonction autoCorrectHitMarkers (transcriptionStore.js)

```javascript
export async function autoCorrectHitMarkers(gameIndex, startMoveIndex = 0) {
    // 1. Construit la position jusqu'à startMoveIndex
    // 2. Pour chaque coup suivant :
    //    - Applique le coup sans étoile
    //    - Vérifie si result.hasHit est true
    //    - Ajoute * si frappe détectée, retire * si pas de frappe
    // 3. Met à jour transcriptionStore avec les notations corrigées
}
```

Caractéristiques :
- Ignore les coups spéciaux : "Cannot Move" et "????"
- Utilise applyMove qui retourne {position, hasHit}
- Met à jour la notation uniquement si changement nécessaire
- Gère à la fois player1 et player2

### 2. Intégration dans invalidatePositionsCacheFrom

```javascript
export async function invalidatePositionsCacheFrom(gameIndex, moveIndex) {
    // 1. Invalide le cache des positions
    positionsCacheStore.update(cache => { ... });
    
    // 2. Auto-corrige les marqueurs de frappe (*)
    await autoCorrectHitMarkers(gameIndex, moveIndex);
    
    // 3. Valide et détecte les incohérences
    validateGameInconsistencies(gameIndex, moveIndex);
}
```

La fonction est maintenant **async** et **await** l'auto-correction avant la validation.

### 3. Simplification de EditMovePanel.svelte

**AVANT** : La fonction validateEditing calculait manuellement si une frappe se produisait et ajoutait * uniquement au coup adverse immédiat suivant.

**APRÈS** : Code simplifié - laisse autoCorrectHitMarkers gérer TOUS les coups suivants automatiquement.

```javascript
async function validateEditing() {
    if (hasChanges) {
        updateMove(gameIndex, move.moveNumber, player, diceInput, moveInput, isIllegal, isGala);
        
        // Cette fonction fait tout automatiquement :
        // - Auto-correction des * pour tous les coups suivants
        // - Validation des incohérences
        await invalidatePositionsCacheFrom(gameIndex, moveIndex);
        
        selectedMoveStore.set({ ...selectedMove });
        statusBarTextStore.set(`Move updated... (auto-correcting subsequent moves...)`);
    }
}
```

## Fichiers modifiés

1. **frontend/src/stores/transcriptionStore.js**
   - Ajout : `autoCorrectHitMarkers(gameIndex, startMoveIndex)`
   - Modification : `invalidatePositionsCacheFrom` devient async et appelle autoCorrectHitMarkers

2. **frontend/src/components/EditMovePanel.svelte**
   - Suppression : Logique manuelle de détection et ajout de * (lignes 119-165)
   - Modification : `validateEditing` devient async et await invalidatePositionsCacheFrom

## Workflow complet après édition d'un coup

1. Utilisateur sélectionne un coup avec Tab
2. Édite les dés ou la notation
3. Appuie sur Entrée pour valider
4. `validateEditing()` :
   - Met à jour le coup édité dans transcriptionStore
   - Appelle `invalidatePositionsCacheFrom(gameIndex, moveIndex)`
5. `invalidatePositionsCacheFrom()` :
   - Invalide le cache des positions
   - Appelle `autoCorrectHitMarkers(gameIndex, moveIndex)`
6. `autoCorrectHitMarkers()` :
   - Reconstruit la position jusqu'au coup édité
   - Pour chaque coup suivant : applique + vérifie hasHit + corrige notation
   - Met à jour transcriptionStore
7. `validateGameInconsistencies()` :
   - Détecte les coups toujours incohérents (règle de la barre, etc.)
   - Met à jour moveInconsistenciesStore
8. Interface :
   - MovesTable affiche les coups avec notation corrigée
   - Coups incohérents surlignés en rouge

## Tests recommandés

### Test 1 : Frappe simple (match2)
1. Charger match2.lbg
2. Éditer move 6 player 2 : `43 15/8` → `42 15/9`
3. Vérifier move 12 player 2 : devrait être `51 Bar/24* 21/16*`
4. Vérifier move 16 : devrait être détecté incohérent (3 pions à la barre)

### Test 2 : Retrait de frappe
1. Éditer un coup pour qu'il NE frappe PLUS
2. Vérifier que * est retiré du coup adverse suivant

### Test 3 : Cascade de corrections
1. Éditer un coup au début d'une partie
2. Vérifier que TOUS les coups suivants sont corrigés si nécessaire

## Avantages de cette approche

✅ **Correction automatique** : Plus besoin de manuellement corriger les notations
✅ **Cohérence garantie** : Tous les coups suivants sont traités
✅ **Code simplifié** : EditMovePanel n'a plus besoin de logique complexe
✅ **Séparation des responsabilités** :
   - EditMovePanel : interface utilisateur
   - transcriptionStore : logique métier d'auto-correction
   - positionCalculator : calcul des positions et détection de frappes

## Compilation

```bash
cd /home/unger/src/lazyBG/frontend
npm run build
```

Build réussi : **308.24 kB | gzip: 82.50 kB**
