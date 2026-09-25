# Guide cloud-init

Cloud-init configure une VM à son premier démarrage sans s'y connecter :
paquets, fichiers, commandes, fuseau horaire, etc. Dans PVMSS, un **document
cloud-init** est un template `#cloud-config` rédigé par votre administrateur
et publié sur le cluster. Vous en choisissez un à la création d'une VM ; vous
n'écrivez jamais de YAML vous-même.

## Choisir un document

Le formulaire **Créer une VM** liste les templates que votre administrateur a
rendus disponibles pour le cluster. Le sélecteur est masqué quand le cluster
n'a pas de publication cloud-init - demandez à votre administrateur si vous
pensiez le voir.

Un template est versionné : quand votre administrateur le modifie, les VM
existantes gardent la version avec laquelle elles ont été créées. Les
nouvelles VM reçoivent la nouvelle version.

## Ce qu'un document peut faire

Le document est livré comme *vendor data* cloud-init, fusionné avec les
réglages du formulaire (utilisateur, mot de passe, clés SSH, réseau). Tout ce
qui passe par le canal vendor s'applique : `packages`, `package_update`,
`package_upgrade`, `runcmd`, `bootcmd`, `write_files`, `apt`, `timezone`,
`ntp`, et le reste du [schéma cloud-init](https://cloudinit.readthedocs.io/).

Une limite connue : une clé `users:` dans le document est écrasée par le
compte généré depuis le formulaire. Mettez les comptes et les clés SSH dans
les champs Cloud-init du formulaire, pas dans le document.

## Changer le document d'une VM

Après la création, l'onglet **Cloud-init** de la VM affiche le document
utilisé. Vous pouvez choisir un autre template ou le détacher. Le changement
est pris en compte au prochain démarrage, mais l'essentiel d'un document ne
s'exécute qu'au premier démarrage d'une nouvelle VM (voir ci-dessous).

## Ce qui s'applique et quand

Les modules cloud-init ne se rejouent pas tous de la même manière :

- **Les réglages réseau** (IP, passerelle, DNS, domaine de recherche) sont
  réappliqués à chaque démarrage - un redémarrage suffit.
- **Le mot de passe** est transmis immédiatement à l'invité en cours
  d'exécution via l'agent QEMU ; aucun redémarrage n'est nécessaire.
- **L'utilisateur, la liste des clés SSH et l'essentiel d'un document** sont
  consommés par des modules « par instance » qui ne s'exécutent **qu'une
  fois, au premier démarrage d'une nouvelle VM**. Les modifier sur une VM
  déjà provisionnée met à jour la configuration mais ne rejoue rien dans
  l'invité.

Pour réappliquer ces changements sur une VM déjà provisionnée, lancez dans
l'invité, puis redémarrez :

```sh
sudo cloud-init clean --logs --seed && sudo reboot
```

Pour ajouter une clé SSH à une VM en marche sans tout cela, utilisez la
section **Ajouter une clé maintenant** de l'onglet Cloud-init : la clé est
injectée immédiatement via l'agent invité et enregistrée dans la configuration
pour les prochains démarrages.

## Les documents ne sont pas un coffre-fort

Les templates sont stockés en clair - dans la base du portail et sur le
stockage de snippets du cluster, où cloud-init doit pouvoir les lire. Ils ne
contiennent jamais vos mots de passe ni vos clés : utilisez le champ **mot de
passe** de cloud-init (transmis via l'agent invité et jamais stocké) et les
clés SSH du formulaire de la VM.
