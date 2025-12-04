# Tests des Frappes et de la Règle de la Barre

## Test 1 : Détection et Marquage Automatique d'une Frappe

### Scénario
Un joueur édite un coup pour frapper un pion adverse.

### Étapes
1. Ouvrir un match avec des coups existants
2. Sélectionner le coup 1, joueur 1
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Modifier le coup pour frapper : `24/20* 13/8` (point 20 a un pion adverse seul)
5. Appuyer sur `Entrée` pour valider

### Résultat Attendu
- Le coup du joueur 1 est enregistré avec la notation saisie
- Le coup suivant du joueur 2 (dans le même tour ou au tour suivant) reçoit automatiquement une étoile (*)
- Message dans la barre d'état : "Move updated at game X, move Y (hit detected, * added)"
- La position du joueur 2 montre qu'il a maintenant un pion à la barre

---

## Test 2 : Validation de la Règle de la Barre - Cas Valide

### Scénario
Un joueur a un pion à la barre et fait correctement entrer ce pion d'abord.

### Étapes
1. Créer une situation où le joueur 1 a un pion à la barre
2. Sélectionner le coup du joueur 1
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Saisir un coup valide commençant par la barre : `52` puis `bar/20 6/4`
5. Appuyer sur `Entrée` pour valider

### Résultat Attendu
- Le coup est accepté et enregistré
- Aucune incohérence détectée
- Le pion entre correctement de la barre
- Le coup n'est pas marqué en rouge

---

## Test 3 : Validation de la Règle de la Barre - Cas Invalide

### Scénario
Un joueur a un pion à la barre mais essaie de jouer ailleurs sans l'entrer d'abord.

### Étapes
1. Créer une situation où le joueur 1 a un pion à la barre
2. Sélectionner le coup du joueur 1
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Saisir un coup INVALIDE qui ne fait pas entrer de la barre : `52` puis `13/8 6/4`
5. Appuyer sur `Entrée` pour valider

### Résultat Attendu
- Le coup est enregistré mais marqué comme INCOHÉRENT
- Le coup apparaît en rouge dans la table des coups
- Message d'erreur au survol : "Must enter from bar before moving other checkers"
- Tous les coups suivants sont aussi marqués incohérents

---

## Test 4 : Frappe Créant une Situation de Barre pour l'Adversaire

### Scénario
Éditer un coup qui frappe un pion, forçant l'adversaire à entrer de la barre.

### Étapes
1. Sélectionner un coup du joueur 1 où il peut frapper
2. Appuyer sur `Tab` pour entrer en mode EDIT
3. Modifier le coup pour frapper : `63` puis `24/18*`
4. Appuyer sur `Entrée` pour valider
5. Observer le coup suivant du joueur 2

### Résultat Attendu
- Le coup du joueur 1 est enregistré
- L'étoile (*) est ajoutée au coup du joueur 2
- Si le coup du joueur 2 ne commence pas par `bar/`, il doit être marqué incohérent
- Message d'erreur sur le coup du joueur 2 : "Must enter from bar before moving other checkers"

---

## Test 5 : Plusieurs Pions à la Barre

### Scénario
Un joueur a plusieurs pions à la barre et doit tous les entrer.

### Étapes
1. Créer une situation où le joueur 1 a 2 pions à la barre
2. Sélectionner le premier coup du joueur 1 après les frappes
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Saisir un coup n'entrant qu'un seul pion : `54` puis `bar/21`
5. Valider et vérifier le coup suivant du joueur 1

### Résultat Attendu
- Le premier coup (entrée d'un pion) est valide
- Le coup suivant du joueur 1 DOIT aussi commencer par `bar/` s'il a encore des pions à la barre
- Sinon, il sera marqué incohérent

---

## Test 6 : Frappe Multiple dans le Même Coup

### Scénario
Un coup frappe plusieurs pions adverses en une seule fois.

### Étapes
1. Sélectionner un coup où deux frappes peuvent être faites
2. Appuyer sur `Tab` pour entrer en mode EDIT
3. Saisir : `66` puis `24/18* 18/12*`
4. Appuyer sur `Entrée` pour valider

### Résultat Attendu
- Le coup est enregistré avec les deux frappes
- L'étoile (*) est ajoutée au coup adverse suivant
- L'adversaire doit avoir 2 pions à la barre
- Le coup adverse doit commencer par entrer ces 2 pions

---

## Test 7 : Suppression de l'Étoile si Plus de Frappe

### Scénario
Modifier un coup qui avait une frappe pour ne plus en avoir.

### Étapes
1. Trouver un coup marqué avec `*` après le mouvement
2. Appuyer sur `Tab` pour entrer en mode EDIT
3. Modifier le coup pour qu'il n'y ait plus de frappe : `64` puis `13/7 8/4`
4. Appuyer sur `Entrée` pour valider

### Résultat Attendu
- Le coup est enregistré sans `*`
- Si le coup suivant de l'adversaire avait `*`, il devrait être recalculé
- L'adversaire ne doit plus avoir de pion à la barre

---

## Test 8 : Cannot Move avec Pion à la Barre

### Scénario
Un joueur a un pion à la barre mais ne peut pas entrer (tous les points d'entrée sont bloqués).

### Étapes
1. Créer une situation où tous les points d'entrée sont bloqués pour le joueur 1
2. Sélectionner le coup du joueur 1
3. Appuyer sur `Tab` pour entrer en mode EDIT
4. Saisir : `35` puis `Cannot Move`
5. Appuyer sur `Entrée` pour valider

### Résultat Attendu
- Le coup "Cannot Move" est accepté
- Aucune incohérence détectée (car c'est une exception valide)
- Le pion reste à la barre pour le tour suivant

---

## Notes de Test

### Vérifications Importantes
- [ ] L'étoile (*) est bien ajoutée automatiquement au coup adverse après frappe
- [ ] L'étoile est retirée lors de la validation interne (fonction `removeHitMarker`)
- [ ] La règle de la barre est strictement appliquée (tous les segments doivent partir de `bar/`)
- [ ] Les coups spéciaux (`Cannot Move`, `????`) ne déclenchent pas d'erreurs
- [ ] Les incohérences se propagent correctement aux coups suivants
- [ ] Le nombre de pions à la barre est correctement calculé

### Cas Limites
- Frappe sur le dernier coup d'un jeu (pas de coup suivant)
- Multiples frappes successives
- Frappe puis bear-off
- Entrée de la barre puis bear-off dans le même coup
