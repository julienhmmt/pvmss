# Guide administrateur PVMSS

Ce guide couvre toutes les fonctionnalités administratives et les workflows disponibles dans PVMSS, incluant la configuration système, la gestion des utilisateurs et la maintenance de l'application.

L'administrateur de l'application PVMSS dispose d'un accès complet à toutes les fonctionnalités de l'application. Il n'y a pas de rôle administrateur distinct, pas de rôle auditeur ou observateur. En naviguant vers la page <http://ip_ou_nom-de-domaine/admin>, vous accéderez à l'interface d'administration après avoir validé la connexion avec le mot de passe administrateur.

## Guide de démarrage

1. Accédez au panneau d'administration sur `/admin` (mot de passe administrateur requis)
2. Créez des tags pour catégoriser les machines virtuelles
3. Examinez et configurez options suivantes que vous souhaitez mettre à disposition lors de la création des machines virtuelles :
    - les stockages qui seront disponibles
    - les images ISO
    - les ponts réseau (vmbr)
    - les limites de ressources (processeur, ram, et taille de disque)
4. Créer autant de comptes utilisateurs que nécessaire
5. Communiquez à vos utilisateurs l'accessibilité de l'application PVMSS pour qu'ils commencent à créer leurs VMs
6. Surveillez les logs de l'application PVMSS pour détecter tout problème

## Configuration de l'application

Dans cette interface, plusieurs rubriques seront accessibles au travers d'un menu de navigation vertical sur la gauche.

### Informations sur l'application (App info)

Cette rubrique (accessible via `/admin/appinfo`) fournit une vue d'ensemble en lecture seule de l'instance PVMSS :

- **Informations de build** : version de l'application, version de Go, système d'exploitation et architecture.
- **Environnement** : mode actuel (`development`, `production` ou `offline`) déterminé par les variables d'environnement `PVMSS_ENV` et `PVMSS_OFFLINE`.
- **Statut du cluster Proxmox** : indication si PVMSS est connecté à un serveur unique ou à un cluster Proxmox, avec le nom du cluster et le nombre de nœuds lorsque ces informations sont disponibles.
- **Variables d'environnement (sous-ensemble sûr)** : configuration non sensible telle que `PROXMOX_URL`, `PROXMOX_VERIFY_SSL`, `PVMSS_ENV`, `PVMSS_OFFLINE`, `PVMSS_SETTINGS_PATH`.

Utilisez cette page pour vérifier rapidement que l'instance fonctionne avec la configuration attendue et est connectée au bon environnement Proxmox.

### Gestion des noeuds

Cette rubrique affiche la liste de tous les hôtes Proxmox VE, avec un affichage présentant la consommation actuelle du CPU et de la mémoire vive. Le statut du serveur (En ligne, hors ligne) est également affiché.

Les informations de nœuds sont rafraîchies par un worker en tâche de fond toutes les 30 secondes (valeur définie par `NodeCacheRefreshInterval`). Les pages d'administration lisent uniquement ce cache local : l'affichage est donc instantané même sur de gros clusters Proxmox, et un nœud lent n'impacte plus la navigation. Les données restent disponibles hors‑ligne tant que la dernière actualisation n'est pas trop ancienne.

### Gestion des tags

Cette rubrique permet de gérer les tags utilisés pour catégoriser les machines virtuelles. Tous les tags créés dans PVMSS sont affichés et peuvent être supprimés. Un tag est immuable. Le tag `pvmss` est un tag par défaut et ne peut pas être supprimé.

De plus, un compteur de machines virtuelles par tag est affiché.

Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"tags": ["pvmss","tag"]}`).

### Gestion des stockages

Cette rubrique permet de gérer les stockages Proxmox utilisés pour héberger les disques des machines virtuelles. Tous les backends de stockage capables de contenir des disques VM sont affichés, regroupés par nœud lorsque PVMSS est connecté à un cluster.

Un bouton "Activer" ou "Désactiver" permet de sélectionner quelles paires `(nœud, stockage)` seront proposées sur la page *Créer une VM*. Activer un stockage ici ne modifie pas la configuration Proxmox sous-jacente ; cela contrôle uniquement ce que l'interface PVMSS expose aux utilisateurs.

Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"enabled_storages": ["nom-noeud:nom_stockage"]}`).

En bas de la page, une section **Configuration des disques** permet de définir le **nombre maximum de disques par VM** (`MaxDiskPerVM`). Cette valeur contrôle le nombre d'emplacements de disque disponibles dans le formulaire *Créer une VM*, en complément des limites techniques de chaque bus disque (VirtIO, SCSI, SATA, IDE).

### Gestion des ISO

Cette rubrique permet de gérer les ISO utilisés pour créer les machines virtuelles. L'interface ne permet pas d'ajouter ni de supprimer des fichiers ISO d'un stockage, mais de sélectionner les ISO qui seront disponibles pour la création des machines virtuelles. Tous les stockages permettant le stockage des fichiers ISO sont parsés et seuls les fichiers ISO sont affichés (un filtre est appliqué, mis en place dans le code).

Un bouton "Activer" ou "Désactiver" permet de sélectionner les ISO qui seront disponibles pour la création des machines virtuelles. Il n'est pas possible de renommer les fichiers ISO au travers de l'interface.

Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"isos": ["nom_stockage:iso/nom_image.iso"]}`).

### Gestion des ponts réseau (VMBR)

Cette rubrique permet de gérer les ponts réseau utilisés pour les machines virtuelles. Tous les ponts réseau créés sur les hôtes Proxmox sont affichés. Les ponts réseau de type "OpenVSwitch" ne sont pas affichés.

Un bouton "Activer" ou "Désactiver" permet de sélectionner quelles paires `(nœud, pont)` seront disponibles lors de la création ou de la modification des machines virtuelles.

Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"vmbrs": ["nom-noeud:nom_pont_reseau"]}`).

En haut de la page, une section **Configuration des cartes réseau** permet de définir le **nombre maximum de cartes réseau par VM** (`MaxNetworkCards`). Cette valeur contrôle le nombre de sections de carte réseau affichées sur la page *Créer une VM*. Elle est actuellement bornée entre 1 et 10.

### Gestion des limites des ressources

Cette rubrique permet de gérer des limites pour les machines virtuelles, pour les nœuds et pour les utilisateurs.

Le formulaire **Limites des VMs** permet de définir le minimum et le maximum de sockets CPU, de cœurs CPU, de mémoire vive et de taille de disque virtuel qu'une nouvelle machine virtuelle peut avoir. Ces limites sont appliquées à la fois sur la page *Créer une VM* et sur le formulaire *Modifier les ressources* de la page de détails de la VM.

Un second formulaire, dédié aux **limites des nœuds**, permet de définir des limites agrégées pour chaque nœud (nombre total de cœurs et quantité totale de mémoire que l'ensemble des VMs gérées par PVMSS peuvent consommer sur ce nœud). La page d'administration affiche également l'utilisation agrégée actuelle par nœud avec des barres de progression, afin de repérer rapidement les nœuds proches de leur capacité configurée.

Les paramètres pour les limites des machines virtuelles sont enregistrés dans un fichier au format JSON (chemin : `{"limits": {"vm": {"cores": {"max": 2, "min": 1}, "disk": {"max": 10, "min": 1}, "ram": {"max": 4, "min": 1}, "sockets": {"max": 1, "min": 1}}}}`).

Les paramètres pour les limites des nœuds sont enregistrés dans un fichier au format JSON (chemin : `{"limits": {"nodes": {"nom-noeud": {"cores": {"max": 8, "min": 2}, "ram": {"max": 32, "min": 2}, "sockets": {"max": 1, "min": 1}}}}}`).

Enfin, une section **Limites utilisateurs** permet de définir le **nombre maximum de VMs par utilisateur** (`max_vm_per_user`). Cette limite globale est stockée avec les autres paramètres (par exemple : `{"max_vm_per_user": 5}`) et est appliquée lorsque les utilisateurs tentent de créer de nouvelles VMs.

### Gestion des utilisateurs

Cette rubrique permet de gérer les utilisateurs de l'application PVMSS. Plutôt que de stocker les utilisateurs dans une base de données, les utilisateurs sont directement créés dans le noeud Proxmox VE, en utilisant l'API mise à disposition.

Un compte utilisateur est composé d'un nom d'utilisateur, d'un royaume, d'un mot de passe et d'un rôle. Le royaume est `@pve` et n'est pas modifiable. Le rôle pour tous les utilisateurs est `PVEVMUser`.

Pour que chaque utilisateur puisse avoir ses VM dans un seul et unique dossier, un pool Proxmox est créé pour chaque utilisateur, dont le nom est composé de `pvmss_` et le nom d'utilisateur.

Par exemple, pour l'utilisateur `essai`, le pool sera `pvmss_essai` et son compte sera `essai@pve`. Il n'est pas possible de modifier le compte utilisateur, mais il est possible de le supprimer. Cette suppression supprimera également le pool Proxmox et toutes les VM associées.

### Comptes administrateur

En plus du compte administrateur intégré (configuré avec `ADMIN_PASSWORD_HASH`), PVMSS supporte les comptes administrateur créés directement dans Proxmox VE. Cela permet à plusieurs administrateurs d'accéder à l'interface admin de PVMSS en utilisant leurs identifiants Proxmox.

#### Création d'un compte administrateur

Pour créer un compte administrateur, utilisez la ligne de commande Proxmox :

```bash
# Créer l'utilisateur dans le royaume 'pve' (obligatoire)
pveum user add admin-user@pve

# Définir le mot de passe
pveum passwd admin-user@pve

# Accorder le rôle PVEAdmin au niveau racine (obligatoire pour l'accès admin complet)
pveum aclmod / -user admin-user@pve -role PVEAdmin
```

**Notes importantes :**

- **Royaume** : Doit être `@pve`, pas `@pam`. Le royaume `pve` est requis pour une intégration correcte avec l'authentification PVMSS.
- **Rôle** : Doit être `PVEAdmin` uniquement. Ce rôle fournit un accès administratif complet à Proxmox et accorde l'accès admin dans PVMSS.
- **Pas de pool** : Contrairement aux utilisateurs normaux, les administrateurs n'ont pas de pool dédié.

Après création, l'utilisateur peut se connecter à PVMSS avec les identifiants `admin-user@pve` et aura automatiquement accès à l'interface d'administration.

## Limites connues

- L'application PVMSS est conçue pour fonctionner sur des serveurs Proxmox VE 9.0 et supérieurs
- Il n'est pas possible de connecter un système d'authentification externe à l'application PVMSS (OIDC, SAML, etc.)
- PVMSS supporte à la fois les serveurs Proxmox standalone et les clusters Proxmox, mais les opérations de cluster avancées (comme la migration à chaud, la haute disponibilité ou l'orchestration des sauvegardes) doivent toujours être réalisées directement depuis l'interface Proxmox.
