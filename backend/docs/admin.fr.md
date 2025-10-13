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

### Gestion des noeuds

Cette rubrique affiche la liste de tous les hôtes Proxmox VE, avec un affichage présentant la consommation actuelle du CPU et de la mémoire vive. Le statut du serveur (En ligne, hors ligne) est également affiché.

### Gestion des tags

Cette rubrique permet de gérer les tags utilisés pour catégoriser les machines virtuelles. Tous les tags créés dans PVMSS sont affichés et peuvent être supprimés. Un tag est immuable. Le tag `pvmss` est un tag par défaut et ne peut pas être supprimé.

De plus, un compteur de machines virtuelles par tag est affiché.

Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"tags": ["pvmss","tag"]}`).

### Gestion des stockages

Cette rubrique permet de gérer les stockages utilisés pour stocker les machines virtuelles. Tous les stockages supportant les fichiers de disque virtuels sont affichés.

Un bouton "Activer" ou "Désactiver" permet de sélectionner les stockages qui seront utilisés pour stocker les machines virtuelles.

Le stockage `*local*` est le stockage par défaut et ne peut pas être utilisé. Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"enabled_storages": ["nom_stockage"]}`).

### Gestion des ISO

Cette rubrique permet de gérer les ISO utilisés pour créer les machines virtuelles. L'interface ne permet pas d'ajouter ni de supprimer des fichiers ISO d'un stockage, mais de sélectionner les ISO qui seront disponibles pour la création des machines virtuelles. Tous les stockages permettant le stockage des fichiers ISO sont parsés et seuls les fichiers ISO sont affichés (un filtre est appliqué, mis en place dans le code).

Un bouton "Activer" ou "Désactiver" permet de sélectionner les ISO qui seront disponibles pour la création des machines virtuelles. Il n'est pas possible de renommer les fichiers ISO au travers de l'interface.

Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"isos": ["nom_stockage:iso/nom_image.iso"]}`).

### Gestion des ponts réseau (VMBR)

Cette rubrique permet de gérer les ponts réseau utilisés pour les machines virtuelles. Tous les ponts réseau créés dans l'hôte Proxmox sont affichés. Les ponts réseau de type "OpenVSwitch" ne sont pas affichés.

Un bouton "Activer" ou "Désactiver" permet de sélectionner les ponts réseau qui seront utilisés pour les machines virtuelles.

Les paramètres sont enregistrés dans un fichier au format JSON (chemin : `{"vmbrs": ["nom_pont_reseau"]}`).

### Gestion des limites des ressources

Cette rubrique permet de gérer des limites pour les machines virtuelles mais aussi pour les noeuds.

Le formulaire pour les limites des machines virtuelles permet de définir le minimum et le maximum de coeur de processeur, de quantité de mémoire vive et de taille de stockage virtuel qu'une nouvelle machine virtuelle peut avoir.

Un second formulaire, dédié pour les limites des noeuds, permet de définir le minimum et le maximum de coeur de processeur et de quantité de mémoire vive qu'un noeud peut supporter. Ce formulaire permet de définir des limites globales pour les noeuds.

Les paramètres pour les limites des machines virtuelles sont enregistrés dans un fichier au format JSON (chemin : `{"limits": {"vm": {"cores": {"max": 2,"min": 1},"disk": {"max": 10,"min": 1},"ram": {"max": 4,"min": 1},"sockets": {"max": 1,"min": 1}}}`).

Les paramètres pour les limites des noeuds sont enregistrés dans un fichier au format JSON (chemin : `{"limits": {"nodes": {"nom-noeud": {"cores": {"max": 8,"min": 2},"ram": {"max": 32,"min": 2},"sockets": {"max": 1,"min": 1}}}}`).

### Gestion des utilisateurs

Cette rubrique permet de gérer les utilisateurs de l'application PVMSS. Plutôt que de stocker les utilisateurs dans une base de données, les utilisateurs sont directement créés dans le noeud Proxmox VE, en utilisant l'API mise à disposition.

Un compte utilisateur est composé d'un nom d'utilisateur, d'un royaume, d'un mot de passe et d'un rôle. Le royaume est `@pve` et n'est pas modifiable. Le rôle pour tous les utilisateurs est `PVEVMUser`.

Pour que chaque utilisateur puisse avoir ses VM dans un seul et unique dossier, un pool Proxmox est créé pour chaque utilisateur, dont le nom est composé de `pvmss_` et le nom d'utilisateur.

Par exemple, pour l'utilisateur `essai`, le pool sera `pvmss_essai` et son compte sera `essai@pve`. Il n'est pas possible de modifier le compte utilisateur, mais il est possible de le supprimer. Cette suppression supprimera également le pool Proxmox et toutes les VM associées.

## Limites connues

- L'application PVMSS est conçue pour fonctionner sur des serveurs Proxmox VE 8.0 et supérieurs
- Il n'est pas possible de connecter un système d'authentification externe à l'application PVMSS (OIDC, SAML, etc.)
- Seul un noeud Proxmox est supporté. Si vous souhaitez gérer plusieurs noeuds Proxmox, vous devrez créer une instance de l'application PVMSS pour chaque noeud Proxmox.
