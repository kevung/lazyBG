.. _guide_utilisateur:

Guide utilisateur
=================

Ce guide est une introduction pratique à blunderDB pour une prise en main
rapide.

Créer une nouvelle base de données
----------------------------------

Pour créer une nouvelle base de données, cliquer dans la barre d'outils sur le
bouton "New Database". Choisir un chemin où enregistrer la base de données,
ainsi qu'un nom et cliquer sur "Save".

.. note::
   L'extension des bases de données blunderDB est *.db*.

.. tip::
   Raccourcis clavier: *CTRL-N*. Commande: ``n``


Ouvrir une base de donnée existante
-----------------------------------

Pour charger une base de données existante, cliquer dans la barre d'outils sur
le bouton "Open Database". Choisir le chemin où se trouve la base de données,
choisir le fichier *.db* et cliquer sur "Open".

.. tip::
   Raccourcis clavier: *CTRL-O*. Commande: ``o``

.. _guide_edit_position:

Editer une position
-------------------

Pour éditer une position, basculer en mode EDIT à l'aide de la touche *TAB*.

Le mode EDIT permet de modifier les décisions de backgammon (les dés et les coups joués)
en utilisant le clavier uniquement. Vous ne pouvez pas modifier directement la position
du plateau avec la souris - la position est mise à jour automatiquement en fonction des
décisions que vous saisissez.

Pour éditer une décision:

* Tapez les valeurs des dés (1-6) ou les lettres de décision (**d** = double, **t** = take, 
  **p** = pass, **r** = resign, **g** = resign gammon, **b** = resign backgammon)
* Utilisez **j**/**k** pour naviguer entre les coups candidats suggérés par gnubg
* Appuyez sur **Espace** pour saisir manuellement un coup en notation
* Appuyez sur **Entrée** pour valider et passer à la décision suivante
* Appuyez sur **Echap** pour quitter le mode EDIT

Ajouter une position à la base de données
-----------------------------------------

Après l'édition de la position précédente, blunderDB est dans le mode EDIT.

Pour enregistrer la position obtenue précédemment, faire *CTRL-S* ou appuiyer
dans la barre d'outils sur le bouton "Save Position".

.. tip:: Depuis le mode EDIT, basculer en mode COMMAND et exécuter: ``w``

Etiqueter une position
----------------------

Pour ajouter un tag *toto* à la position courante, basculer en mode COMMAND en appuyant sur *ESPACE*,
taper ``#toto`` et valider la commande en appuyant sur *ENTREE*.

Supprimer une position
----------------------

Pour supprimer la position courante de la base de données, faire *Del* ou
clicker dans la barre d'outils sur le bouton "Delete Position"

.. tip:: En mode COMMAND, exécuter ``d``.

.. caution:: La suppression de la position est définitive et ne nécessite
   aucune confirmation de la part de l'utilisateur.

Import une position depuis XG
-----------------------------

Pour importer une position directement depuis XG,

#. afficher dans XG la position à importer et appuyer *CTRL-C*,

#. afficher blunderDB et appuyer *CTRL-V*.

Afficher l'analyse d'une position importée depuis XG
----------------------------------------------------

Si une position analysée par XG a été importée dans blunderDB, l'analyse de XG
peut être affichée en appuyant *CTRL-L*.

Si la position correspond à une décision de pions, les cinq meilleurs coups
sont affichés sur des lignes distinctes. Pour chaque ligne, les informations
fournies sont dans cet ordre, le coup de pion associé, l'équité normalisée,
l'erreur en équité du coup, les chances de gain, gammon et backgammon du
joueur, les chances de gain, gammon et backgammon de l'adversaire, le niveau
d'analyse. 

Si la position correspond à une décision de cube, le coût de chaque décision
est affiché ainsi que les chances de gain de la position.

Exporter une position vers XG
-----------------------------

Pour exporter une position de blunderDB vers XG,

#. afficher dans blunderDB la position à exporter et appuyter *CTRL-C*,

#. afficher XG et appuyer *CTRL-V*.

Visualiser les différentes positions
------------------------------------

Pour visualiser les différentes positions de la bibliothèque courante, utiliser
les touches *GAUCHE* et *DROITE*. La touche *HOME* permet d'aller à la première
position. La touche *FIN* permet d'aller à la dernière position.

Pour afficher le bearoff à gauche, appuyer *CTRL-GAUCHE*. Pour afficher le
bearoff à droite, appuyer *CTRL-DROITE*.

Rechercher des positions selon des critères
-------------------------------------------

Pour rechercher des types de positions,

* basculer en mode EDIT en appuyant sur *TAB*,

* éditer la structure de position à rechercher. blunderDB va filtrer les
  positions ayant *a minima* la structure de pions saisie. Dans le
  doute, afin d'obtenir le maximum de résultats, effacer la position
  en appuyant sur la touche *RETOUR ARRIERE*. Editer si besoin la
  position du cube et le score.

Méthode 1 (simple): 

* Ouvrir la fenêtre de recherche (*CTRL-F*)

* Ajouter et paramétrer les filtres de recherche

* Valider en cliquant sur "Search".

Méthode 2 (avancée):


* basculer en mode COMMAND en appuyant sur *ESPACE*,

* écrire *s*, ajouter d'éventuels filtres supplémentaires (par exemple
  *cube* ou *score* pour prendre respectivement en compte le cube et le
  score. Voir :numref:`cmd_filter` pour une liste exhaustive des
  filtres disponibles).

* valider la requête en appuyant sur *ENTREE*.

Les positions affichées sont celles de la base de données ayant vérifié
les critères de recherche entrés par l'utilisateur.

