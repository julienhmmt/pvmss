# Guide cloud-init

Cloud-init configure une VM à son premier démarrage sans s'y connecter :
paquets, fichiers, commandes, fuseau horaire, etc. Dans PVMSS, un **document
cloud-init** est un fichier `#cloud-config` que PVMSS copie sur le cluster et
attache à votre VM lors de sa création.

## Deux types de documents

- **Templates admin** — rédigés et validés par votre administrateur, listés
  en premier dans le sélecteur.
- **Mes fichiers** — vos propres documents, gérés sur la page
  [Fichiers cloud-init](/cloud-init) (20 au maximum). Vous seul pouvez les
  voir et les utiliser.

Les deux apparaissent dans un seul sélecteur groupé du formulaire **Créer une
VM**. Le sélecteur est masqué quand le cluster cible n'a pas de cible
d'écriture cloud-init — demandez à votre administrateur si vous pensiez le
voir.

## Votre VM garde sa propre copie

À la création, PVMSS écrit le document choisi sur le cluster sous le nom
`pvmss-<vmid>.yml` et l'attache à la VM. Cette copie appartient à la VM :
modifier le template ou votre fichier ensuite ne change **pas** les VM déjà
créées.

## Ce qu'un document peut faire

Le document est livré comme *vendor data* cloud-init, fusionné avec les
réglages du formulaire (utilisateur, mot de passe, clés SSH, réseau). Tout ce
qui passe par le canal vendor s'applique : `packages`, `package_update`,
`package_upgrade`, `runcmd`, `bootcmd`, `write_files`, `apt`, `timezone`,
`ntp`, et le reste du [schéma cloud-init](https://cloudinit.readthedocs.io/).

Une limite connue : une clé `users:` dans le document est écrasée par le
compte généré depuis le formulaire. Mettez les comptes et les clés SSH dans
les champs Cloud-init du formulaire, pas dans le document.

## Modifier le document d'une VM

Après la création, l'onglet **Cloud-init** de la VM affiche le document. Si
votre administrateur autorise le YAML libre dans la politique, vous pouvez le
modifier et l'enregistrer : l'enregistrement écrase le fichier propre à la VM
et prend effet au prochain démarrage. Enregistrer un document vide le détache.

## Ce qui s'applique et quand

Les modules cloud-init ne se rejouent pas tous de la même manière :

- **Les réglages réseau** (IP, passerelle, DNS, domaine de recherche) sont
  réappliqués à chaque démarrage — un redémarrage suffit.
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

Le contenu est stocké en clair — dans la base du portail et sur le stockage
de snippets du cluster, où cloud-init doit pouvoir le lire — et tout
administrateur peut le consulter. N'y mettez jamais de mots de passe, de
tokens d'API ni de clés privées ; utilisez le champ **mot de passe** de
cloud-init (transmis via l'agent invité et jamais stocké) et des clés SSH.
