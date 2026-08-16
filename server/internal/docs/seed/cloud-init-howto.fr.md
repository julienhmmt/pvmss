# Guide cloud-init

Cloud-init permet de configurer une VM au premier démarrage sans ouvrir de
session : utilisateurs, clés SSH, paquets, et plus encore. PVMSS propose des
modèles cloud-init curés par les administrateurs que vous pouvez choisir au
moment de la création, et permet d'attacher un snippet personnalisé par VM.

## Utiliser un modèle

Dans le formulaire **Créer une VM**, choisissez un modèle dans la liste
déroulante cloud-init. Le contenu du modèle est appliqué tel quel ; vous n'avez
pas besoin de le modifier.

## Modifier le snippet d'une VM

Après la création, ouvrez l'onglet **Cloud-init** de la VM pour consulter ou
modifier le snippet. L'éditeur accepte tout document `#cloud-config` valide. Les
modifications sont poussées vers le stockage cloud-init du cluster et prennent
effet au prochain démarrage.

## Champs pris en charge

Le portail vérifie que votre snippet commence par `#cloud-config`. Les champs
courants incluent :

- `packages` — une liste de paquets à installer.
- `users` — comptes utilisateurs avec clés SSH.
- `write_files` — contenu de fichier arbitraire écrit avant le premier démarrage.
- `runcmd` — une liste de commandes exécutées au premier démarrage.

Consultez la [documentation cloud-init](https://cloudinit.readthedocs.io/) amont
pour le schéma complet.
