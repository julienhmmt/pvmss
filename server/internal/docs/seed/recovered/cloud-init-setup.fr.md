# Configuration cloud-init (administrateur)

Ce guide explique comment cloud-init fonctionne dans PVMSS et comment les
administrateurs préparent cloud-init pour leurs utilisateurs. Cloud-init
configure une VM au premier démarrage sans connexion : utilisateurs, clés SSH,
paquets, et plus.

## Fonctionnement

- Les administrateurs gèrent des **modèles cloud-init** dans le panneau
  d'administration (`/admin/cloudinit-templates`). Un modèle est un document
  `#cloud-config` écrit et validé par l'administrateur. Les modèles sont
  stockés dans la base de données PVMSS, pas sur le stockage Proxmox.
- Lors de la création d'une VM, l'utilisateur choisit un modèle dans le menu
  cloud-init. Son contenu est appliqué à l'identique sur le lecteur cloud-init
  de la nouvelle VM.
- Après création, l'utilisateur peut ouvrir l'onglet **Cloud-init** de la VM
  pour voir ou éditer le snippet. L'éditeur accepte tout document
  `#cloud-config` valide. Les changements sont poussés vers le stockage
  cloud-init du cluster et prennent effet au prochain démarrage.

## Tâches de l'administrateur

1. Ouvrez **Admin > Modèles cloud-init**.
2. Créez un modèle avec un libellé et le contenu `#cloud-config`.
3. Validez que le contenu commence par `#cloud-config` et contient du YAML
   valide. Le portail valide l'en-tête et la syntaxe YAML ; il ne valide pas la
   sémantique cloud-init.
4. Activez le modèle pour qu'il apparaisse dans le menu de création de
   l'utilisateur. Désactivez-le pour le masquer sans le supprimer.

Les modèles sont statiques. Les valeurs spécifiques à l'utilisateur (utilisateur,
mot de passe, clés SSH, réseau) sont définies via les champs cloud-init de la VM
ou en éditant le snippet après la création.

## Champs pris en charge

Champs courants utilisables dans un modèle :

- `packages` — une liste de paquets à installer.
- `users` — comptes utilisateurs avec clés SSH autorisées.
- `write_files` — contenu de fichier arbitraire écrit avant le premier démarrage.
- `runcmd` — une liste de commandes exécutées au premier démarrage.

Voir la [documentation cloud-init](https://cloudinit.readthedocs.io/) pour le
schéma complet.

## Exemples de modèles

Configuration utilisateur de base :

```yaml
#cloud-config
users:
  - name: admin
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-ed25519 AAAAC3... user@example.com
```

Installation de paquets :

```yaml
#cloud-config
package_update: true
package_upgrade: true
packages:
  - vim
  - htop
  - curl
  - git
```

Exécution de script personnalisé :

```yaml
#cloud-config
runcmd:
  - echo "Bonjour depuis cloud-init" > /tmp/bonjour.txt
  - systemctl enable --now docker
```

## Dépannage

- **Modèle non appliqué** : vérifiez que le contenu commence par `#cloud-config`
  et que le lecteur cloud-init de la VM est présent. Consultez le journal
  `/var/log/cloud-init.log` dans l'invité.
- **Changements non pris en compte** : cloud-init s'exécute au démarrage.
  Redémarrez la VM après édition du snippet pour que la nouvelle configuration
  s'applique.
- **YAML invalide** : le portail rejette les modèles qui ne sont pas du YAML
  valide ou qui omettent l'en-tête `#cloud-config`.

## Limitations

- Les modèles sont statiques ; il n'y a pas de variables de modèle. Les valeurs
  spécifiques à l'utilisateur doivent être définies via les champs cloud-init de
  base ou en éditant le snippet par VM.
- Seule la syntaxe YAML et l'en-tête `#cloud-config` sont validées, pas la
  sémantique cloud-init.
