# Correction : Marqueurs de frappe (*) par segment

## Problème identifié

Dans match2 game 1, move 12 player 2 : `51: Bar/24* 21/16`

Le coup comporte **deux segments** :
- `Bar/24` - entre de la barre et frappe
- `21/16` - déplace aussi et peut frapper

Mais la notation ne montrait qu'**un seul** `*` alors que les DEUX segments frappaient.

### Cause du problème

L'ancienne implémentation :
- `applyMove` retournait seulement `hasHit: true/false` (booléen global)
- `addHitMarker` ajoutait simplement `*` à la fin du coup complet
- Impossible de savoir **quels segments spécifiques** avaient frappé

## Solution implémentée

### 1. Modifier applyMove pour tracker les segments qui frappent

**Avant** :
```javascript
export function applyMove(position, moveText, isPlayer = true) {
  const segments = parseMoveNotation(moveText);
  let currentPos = position;
  let hasHit = false;
  
  for (const segment of segments) {
    const result = applyMoveSegment(currentPos, segment, isPlayer);
    currentPos = result.position;
    if (result.hit) {
      hasHit = true;
    }
  }
  
  return { position: currentPos, hasHit };
}
```

**Après** :
```javascript
export function applyMove(position, moveText, isPlayer = true) {
  const segments = parseMoveNotation(moveText);
  let currentPos = position;
  let hasHit = false;
  const hitSegments = []; // Track WHICH segments caused hits
  
  for (let i = 0; i < segments.length; i++) {
    const result = applyMoveSegment(currentPos, segments[i], isPlayer);
    currentPos = result.position;
    if (result.hit) {
      hasHit = true;
      hitSegments.push(i); // Store the index
    }
  }
  
  return { position: currentPos, hasHit, hitSegments };
}
```

### 2. Améliorer addHitMarker pour cibler des segments spécifiques

**Avant** :
```javascript
export function addHitMarker(moveText) {
  if (moveText.includes('*')) {
    return moveText;
  }
  return moveText + '*'; // Ajoute * à la fin seulement
}
```

**Après** :
```javascript
export function addHitMarker(moveText, hitSegments = null) {
  if (!moveText || moveText === 'Cannot Move' || moveText === '????') {
    return moveText;
  }
  
  // Legacy behavior: si pas de segments spécifiés, ajoute * à la fin
  if (!hitSegments || hitSegments.length === 0) {
    if (moveText.includes('*')) {
      return moveText;
    }
    return moveText + '*';
  }
  
  // Nouvelle logique : ajoute * aux segments spécifiques
  const segments = moveText.split(' ').filter(s => s.length > 0);
  
  const markedSegments = segments.map((seg, idx) => {
    const cleanSeg = seg.replace(/\*/g, ''); // Retire * existants
    return hitSegments.includes(idx) ? cleanSeg + '*' : cleanSeg;
  });
  
  return markedSegments.join(' ');
}
```

### 3. Mettre à jour autoCorrectHitMarkers pour utiliser hitSegments

**Avant** :
```javascript
const correctNotation = result.hasHit ? addHitMarker(cleanMove) : cleanMove;
```

**Après** :
```javascript
const correctNotation = result.hasHit 
    ? addHitMarker(cleanMove, result.hitSegments) 
    : cleanMove;
```

## Exemple de fonctionnement

### Cas match2 move 12 player 2

Position avant le coup :
- Pion de player 2 à la barre
- Pion de player 1 sur point 24
- Pion de player 1 sur point 16

Coup : `51: Bar/24 21/16` (notation sans *)

1. **Segment 0 : `Bar/24`**
   - Entre de la barre au point 24
   - Frappe le pion de player 1 sur point 24
   - `result.hit = true` → ajoute index 0 à `hitSegments`

2. **Segment 1 : `21/16`**
   - Déplace du point 21 au point 16
   - Frappe le pion de player 1 sur point 16
   - `result.hit = true` → ajoute index 1 à `hitSegments`

3. **Auto-correction**
   - `hitSegments = [0, 1]` (les deux segments ont frappé)
   - `addHitMarker("Bar/24 21/16", [0, 1])`
   - Résultat : `Bar/24* 21/16*` ✅

## Compatibilité

- ✅ `applyMove` retourne toujours `hasHit` (rétrocompatible)
- ✅ `addHitMarker` sans 2e paramètre fonctionne comme avant (legacy)
- ✅ Tous les usages existants dans App.svelte utilisent seulement `.position`
- ✅ Pas d'impact sur les autres composants

## Fichiers modifiés

1. **frontend/src/utils/positionCalculator.js**
   - `applyMove` : ajout de `hitSegments` au retour
   - `addHitMarker` : ajout paramètre `hitSegments` et logique par segment

2. **frontend/src/stores/transcriptionStore.js**
   - `autoCorrectHitMarkers` : utilise `result.hitSegments` lors de l'appel à `addHitMarker`

## Test recommandé

1. Charger match2.lbg
2. Éditer move 6 player 2 : `43 15/8` → `42 15/9`
3. Vérifier move 12 player 2 :
   - **Avant** : `51 Bar/24* 21/16` (1 seul *)
   - **Après** : `51 Bar/24* 21/16*` (2 frappes marquées) ✅

## Compilation

```bash
npm run build
```

Build réussi : **308.46 kB | gzip: 82.58 kB**
