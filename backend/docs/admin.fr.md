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

#### Exemple de workflow : diagnostiquer la connexion Proxmox et le mode hors‑ligne

1. Ouvrez `/admin/appinfo` et vérifiez le badge **Environnement** :
   - `production` indique un fonctionnement normal en ligne,
   - `development` indique un environnement de développement,
   - `offline` indique que `PVMSS_OFFLINE=true` et que les appels à l'API Proxmox sont volontairement désactivés.
2. Contrôlez le tableau des **variables d'environnement** pour `PROXMOX_URL`, `PROXMOX_VERIFY_SSL`, `PVMSS_ENV` et `PVMSS_OFFLINE` afin de vous assurer qu'elles correspondent à la configuration attendue.
3. Dans la section **Informations Proxmox**, vérifiez si PVMSS détecte un mode cluster ou autonome et combien de nœuds sont visibles.
4. Si l'application signale un mode hors‑ligne ou une impossibilité de joindre Proxmox, consultez les logs du backend PVMSS sur l'hôte (voir "Journalisation et diagnostic" ci‑dessous) pour identifier les erreurs de connexion ou un éventuel problème de configuration.
5. Une fois la configuration corrigée (variables d'environnement, connectivité réseau, permissions Proxmox), redémarrez PVMSS et actualisez `/admin/appinfo` pour confirmer l'environnement et l'état du cluster attendus.

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

#### Exemple de workflow : ajuster les limites lorsqu'un utilisateur ou un nœud est saturé

1. À partir de `/admin/vms` et de la rubrique **Nœuds**, identifiez le nœud ou l'utilisateur qui approche des limites de CPU, de mémoire ou du nombre de VMs (par exemple un nœud très chargé ou un utilisateur avec de nombreuses VMs).
2. Ouvrez `/admin/limits` et analysez les **limites de VM**, les **limites de nœud** et les **limites utilisateurs** actuellement configurées.
3. Décidez si vous devez :
   - Augmenter ou réduire les limites globales par VM (CPU, RAM, disque),
   - Serrer ou assouplir les limites agrégées pour le ou les nœuds concernés,
   - Ajuster `max_vm_per_user` pour l'ensemble des utilisateurs.
4. Appliquez les modifications et enregistrez la configuration. PVMSS appliquera immédiatement les nouvelles limites pour les prochaines créations de VMs et les futures modifications de ressources.
5. Informez les utilisateurs impactés des nouvelles limites et, si nécessaire, demandez-leur d'arrêter ou de supprimer certaines VMs pour se conformer à la politique mise à jour.

### Vue globale des VMs (Admin VMs)

Cette rubrique (accessible via `/admin/vms`) fournit une vue d'ensemble, au niveau du cluster, de toutes les VMs connues de PVMSS :

- Un **badge récapitulatif** affiche le nombre total de VMs sur l'ensemble des nœuds.
- Un **tableau** liste chaque VM avec son VMID, son nom, son nœud, son état et ses tags.
- Un **bouton d'action** sur chaque ligne ouvre la page de détails de la VM, où les actions de cycle de vie (démarrer, arrêter, éteindre, reset, console) et la modification des ressources (quand la VM est arrêtée) sont disponibles.
- Un **lien vers la page de recherche** permet aux administrateurs de basculer vers des filtres plus avancés.

Les administrateurs ne peuvent pas créer de nouvelles VMs depuis cette page ; il s'agit exclusivement d'une vue de supervision et de navigation sur les VMs existantes.

#### Exemple de workflow : auditer les VMs d'un utilisateur donné

1. Ouvrez `/admin/userpool` et localisez l'utilisateur que vous souhaitez auditer. Notez le nom du pool Proxmox (par exemple `pvmss_formation`).
2. Demandez à l'utilisateur, si besoin, de vous fournir les noms ou les tags des VMs qu'il a créées.
3. Ouvrez `/admin/vms` et utilisez le tableau, éventuellement combiné avec la page **Recherche**, pour retrouver ces VMs par VMID, nom, tags ou nœud.
4. Pour chaque VM, ouvrez la page de **Détails de la VM** afin de vérifier sa configuration (CPU, mémoire, disques, réseau, EFI/TPM) et les dernières actions réalisées.
5. Si vous identifiez des problèmes (par exemple trop de VMs sur un même nœud ou des ressources mal dimensionnées), coordonnez-vous avec l'utilisateur et ajustez les limites ou la configuration des VMs si nécessaire.

### Gestion des utilisateurs

Cette rubrique permet de gérer les utilisateurs de l'application PVMSS. Plutôt que de stocker les utilisateurs dans une base de données, les utilisateurs sont directement créés dans le noeud Proxmox VE, en utilisant l'API mise à disposition.

Un compte utilisateur est composé d'un nom d'utilisateur, d'un royaume, d'un mot de passe et d'un rôle. Le royaume est `@pve` et n'est pas modifiable. Le rôle pour tous les utilisateurs est `PVEVMUser`.

Pour que chaque utilisateur puisse avoir ses VM dans un seul et unique dossier, un pool Proxmox est créé pour chaque utilisateur, dont le nom est composé de `pvmss_` et le nom d'utilisateur.

Par exemple, pour l'utilisateur `essai`, le pool sera `pvmss_essai` et son compte sera `essai@pve`. Il n'est pas possible de modifier le compte utilisateur, mais il est possible de le supprimer. Cette suppression supprimera également le pool Proxmox et toutes les VM associées.

### Gestion du User Pool

Cette rubrique (accessible via `/admin/userpool`) fournit une interface pratique pour gérer les **pools `pvmss_*`** et les utilisateurs correspondants :

- Un **formulaire de création** permet de créer un nouvel utilisateur en indiquant un nom d'utilisateur, un mot de passe et un commentaire optionnel.
- Chaque utilisateur créé est stocké directement dans Proxmox avec le rôle `PVEVMUser` et associé à un pool dédié nommé `pvmss_<username>`.
- Un **tableau des pools existants** affiche, pour chaque utilisateur, le nom du pool Proxmox, le commentaire éventuel et le nombre de VMs dans ce pool.
- Pour chaque pool, vous pouvez **rafraîchir** le nombre de VMs et **supprimer** le pool ainsi que le compte utilisateur (ce qui supprime également toutes les VMs associées).

Cette page est le point d'entrée principal pour la gestion des utilisateurs self‑service de PVMSS.

#### Exemple de workflow : créer un utilisateur self‑service et suivre ses VMs

1. Ouvrez `/admin/userpool` et créez un nouvel utilisateur en renseignant le nom d'utilisateur, le mot de passe et, si besoin, un commentaire, puis validez le formulaire.
2. Communiquez à cet utilisateur l'URL de PVMSS ainsi que son nom d'utilisateur et son mot de passe.
3. Demandez à l'utilisateur de se connecter à PVMSS puis d'utiliser la page **Créer une VM** pour créer une ou plusieurs VMs dans son pool dédié `pvmss_<username>`.
4. En tant qu'administrateur, ouvrez `/admin/vms` pour retrouver les VMs nouvellement créées dans la vue globale, ou utilisez la page **Recherche** pour filtrer par VMID, nom, tags ou nœud.
5. Si nécessaire, ajustez les **limites de VM**, les **limites de nœud** ou les **limites utilisateurs** dans la rubrique **Limites** afin de contrôler les ressources que cet utilisateur et ses VMs peuvent consommer.

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

#### Capacités de l'administrateur sur les VMs des utilisateurs

- Les comptes administrateur ne sont **pas** associés à un pool PVMSS dédié et ne disposent donc pas d'un flux "Créer une VM" dans l'interface d'administration.
- Pour créer des VMs en self‑service, un administrateur doit soit :
  - Se connecter en tant qu'utilisateur PVMSS classique (avec son propre pool `pvmss_*`), soit
  - Créer et gérer les VMs directement depuis l'interface Proxmox.
- Depuis l'interface admin PVMSS, les administrateurs peuvent **voir et gérer les VMs des utilisateurs** via la page *Recherche* et la page *Admin VMs* :
  - Ouvrir la page de détails de n'importe quelle VM.
  - Utiliser les mêmes actions de cycle de vie et de modification de ressources que celles exposées aux utilisateurs finaux (sous réserve des permissions Proxmox).

## Journalisation et diagnostic

PVMSS utilise son propre système de logs (configuré via la variable d'environnement `LOG_LEVEL`). Ces logs sont écrits localement par le backend PVMSS et ne sont **pas** envoyés à Proxmox ni à un système de journalisation externe par défaut.

- **Portée des logs PVMSS** :
  - Événements de démarrage et d'arrêt de l'application.
  - Traitement des requêtes HTTP et erreurs internes.
  - Appels à l'API Proxmox effectués par PVMSS et échecs associés.
  - Tâches de fond (rafraîchissement du cache, cache de l'agent invité, etc.).
- **Pas une solution de supervision Proxmox** :
  - Seuls les messages d'erreur et d'avertissement renvoyés par Proxmox lors des appels d'API apparaissent dans les logs PVMSS.
  - PVMSS ne collecte ni n'expose les logs complets du cluster Proxmox, ni syslog, ni l'historique des tâches.
  - Pour déboguer Proxmox lui‑même (nœuds, stockage, cluster, HA, etc.), vous devez continuer à utiliser l'interface et les logs Proxmox.

Pour investiguer un problème, combinez toujours les informations de `/admin/appinfo`, des rubriques **Nœuds** et **Limites**, ainsi que les logs PVMSS locaux sur le serveur. Pour tout problème plus profond côté Proxmox, référez‑vous directement aux outils et journaux Proxmox.

## Limites connues

- L'application PVMSS est conçue pour fonctionner sur des serveurs Proxmox VE 9.0 et supérieurs
- Il n'est pas possible de connecter un système d'authentification externe à l'application PVMSS (OIDC, SAML, etc.)
- PVMSS supporte à la fois les serveurs Proxmox standalone et les clusters Proxmox, mais les opérations de cluster avancées (comme la migration à chaud, la haute disponibilité ou l'orchestration des sauvegardes) doivent toujours être réalisées directement depuis l'interface Proxmox.
- Il n'existe **aucun système de templates de VM intégré** ni de catalogue de modèles réutilisables dans PVMSS. Les templates de VM et les workflows de clonage avancés doivent être gérés directement dans Proxmox.
- PVMSS n'intègre **pas encore cloud-init**. La configuration cloud-init (user-data, injection de clés SSH, etc.) doit être réalisée directement dans Proxmox ou au sein du système invité.
- Les administrateurs **ne peuvent pas créer de VMs depuis l'interface d'administration** ; la création de VMs est uniquement disponible via l'interface self‑service des utilisateurs ou directement dans Proxmox.
