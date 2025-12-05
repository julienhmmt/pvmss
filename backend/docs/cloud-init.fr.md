# Support cloud-init dans PVMSS

## Vue d'ensemble

PVMSS fournit un support cloud-init pour les VMs Proxmox, permettant aux utilisateurs d'appliquer des configurations cloud-init avancées lors de la création de VM. Cette fonctionnalité est optionnelle et nécessite une configuration supplémentaire en raison des limitations de l'API Proxmox.

## Pourquoi cloud-init est optionnel et nécessite une configuration

### Limitations de l'API Proxmox

L'API Proxmox présente des limitations significatives concernant les téléchargements de snippets cloud-init :

- L'endpoint standard `/nodes/{node}/storage/{storage}/upload` avec `content=snippets` n'est pas supporté dans de nombreuses versions de Proxmox
- Même lorsque le stockage de snippets est configuré et activé, l'API peut retourner des erreurs HTTP 400 "bad request"
- Cette limitation affecte tous les utilisateurs Proxmox indépendamment de leur configuration de stockage

### Solution de contournement : téléchargement SSH

En raison de ces limitations d'API, PVMSS implémente le téléchargement de snippets cloud-init via SSH, de manière similaire à la façon dont les fournisseurs Terraform populaires (Telmate et bpg/proxmox) gèrent cette limitation :

- **Fournisseur Telmate** : utilise SSH/SCP pour télécharger directement les snippets dans `/var/lib/vz/snippets/`
- **Fournisseur bpg** : utilise SFTP avec des comptes PAM, documenté comme "Snippets Cannot Be Uploaded by Non-PAM Accounts"

Vous pouvez en savoir plus sur cette approche dans la documentation du fournisseur Terraform bpg : <https://registry.terraform.io/providers/bpg/proxmox/latest/docs#ssh-connection>

Cette approche nécessite :

1. Un compte PAM dédié sur les nœuds Proxmox
2. Une authentification par clé SSH
3. Des permissions de système de fichiers appropriées pour le répertoire des snippets

## Configuration requise

### Créer un compte PAM sur le nœud Proxmox utilisé par PVMSS

En tant qu'utilisateur `root@pam`, exécutez :

```bash
# Créer un utilisateur dédié (pas de login shell)
useradd --create-home --shell /usr/sbin/nologin pvmss-snippets

# Créer le répertoire .ssh et définir les permissions
mkdir -p /home/pvmss-snippets/.ssh
chmod 700 /home/pvmss-snippets/.ssh
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh

# Ajouter la clé publique SSH (remplacer par votre clé publique réelle)
echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... votre-clé-publique-ici' | tee /home/pvmss-snippets/.ssh/authorized_keys
chmod 600 /home/pvmss-snippets/.ssh/authorized_keys
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh/authorized_keys
```

### Définir les permissions du système de fichiers

```bash
# Pour le stockage 'local', les snippets sont typiquement dans /var/lib/vz/snippets
apt install acl
setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Vérifier les permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test-from-pvmss
```

## Fonctionnement

1. **Sélection de modèle** : l'utilisateur sélectionne un modèle cloud-init dans le formulaire de création de VM
2. **Téléchargement du snippet** : PVMSS télécharge le contenu YAML via SFTP vers le nœud Proxmox
3. **Configuration de la VM** : PVMSS définit le paramètre `cicustom` pour référencer le snippet téléversé
4. **Processus de démarrage** : Proxmox crée un disque cloud-init avec la configuration personnalisée

## Considérations de sécurité

- **Compte dédié** : le compte `pvmss-snippets` n'a pas d'accès shell et est restreint à SFTP
- **Permissions minimales** : le compte n'a qu'un accès en écriture au répertoire des snippets
- **Authentification par clé SSH** : aucun mot de passe n'est stocké ou transmis

## Dépannage

### Problèmes courants

1. **Permission refusée** : vérifiez les permissions du système de fichiers sur le répertoire des snippets
2. **Échec de l'authentification SSH** : vérifiez que la clé SSH est correctement configurée et accessible
3. **Snippet introuvable** : assurez-vous que le stockage supporte le type de contenu snippets
4. **Problèmes réseau** : vérifiez que PVMSS peut atteindre le nœud Proxmox sur le port 22

## Limitations

- **Connexion SSH requise** : nécessite un accès SSH aux nœuds Proxmox pour le téléchargement des snippets
- **Nœud unique** : supporte actuellement le téléchargement de snippets vers un seul nœud Proxmox

## Approches alternatives

Si vous ne pouvez pas utiliser l'approche SSH :

1. **Snippets manuels** : créez manuellement les snippets sur les nœuds Proxmox et référencez-les dans PVMSS (ajoutez le chemin vers le snippet dans le paramètre cicustom, par exemple `user=local:snippets/mon-modele.yaml`)
2. **Cloud-init de base** : utilisez uniquement les paramètres cloud-init de base (ciuser, cipassword, ipconfig0)
3. **Configuration externe** : utilisez des outils de gestion de configuration externes comme Ansible
