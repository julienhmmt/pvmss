# Guide utilisateur

PVMSS (Proxmox Virtual Machine Self-Service) est un portail en libre-service
qui vous permet de créer, gérer et accéder aux consoles de machines virtuelles
hébergées sur Proxmox VE, sans passer par l'interface Proxmox.

## Démarrage rapide

1. **Connectez-vous** sur la [page de connexion](/login) : choisissez votre cluster puis saisissez les identifiants fournis par votre administrateur.
2. **Retrouvez vos VM** sur la page Mes VM ; recherchez par nom, VMID ou tag.
3. **Créez une VM** avec le bouton « Créer une VM », puis renseignez les paramètres.
4. **Ouvrez la console** une fois la VM créée et démarrée, via le client noVNC ou série intégré.

## Accueil

Après connexion, vous arrivez sur **Mes VM**. Votre consommation de quota est
affichée dans la barre latérale. Chaque opération
longue (création, suppression, snapshot…) apparaît dans le tiroir des tâches
en haut de page : vous pouvez naviguer pendant qu'elle s'exécute.

## Créer une machine virtuelle

Ouvrez l'assistant via « Créer une VM » après connexion. Le mode **Simple**
ne demande que l'essentiel ; le mode **Détaillé** expose toutes les options.
Configurez :

- **Source** : une image **ISO** ou un **template** Proxmox à cloner - tous deux issus des listes approuvées par l'administrateur. Un clone reste sur le nœud du template et garde les accès intégrés au template (voir **Se connecter à sa VM** plus bas).
- **Nom et description** : un nom en minuscules, avec tirets, unique dans votre pool. Un nom clair (`web-prod-01`) rend la liste cherchable et le journal d'activité lisible.
- **Cluster et nœud** : le nœud est choisi automatiquement (nœud approuvé le moins chargé avec assez de stockage) sauf si vous en choisissez un en mode Détaillé.
- **Profil (optionnel)** : si votre administrateur a publié des profils matériels, choisissez-en un pour remplir CPU, mémoire et disque.
- **Ressources** : sockets, cœurs, mémoire et taille de disque. Les valeurs sont bornées par la politique du cluster et votre quota.
- **Stockage** : un stockage approuvé ; l'assistant vérifie l'espace libre en direct.
- **Réseau** : une ou plusieurs cartes, chacune avec un bridge et un modèle (VirtIO, E1000, RTL8139, VMXNet3). Le pare-feu Proxmox est toujours activé ; votre administrateur peut imposer un VLAN d'isolation.
- **Firmware** : UEFI (activé par défaut) avec un magasin de clés EFI vide - Secure Boot n'est jamais activé, car la plupart des ISO Linux ne sont pas signés et ne démarreraient pas avec. TPM 2.0 pour les invités qui l'exigent.
- **Démarrage** : choisissez si la VM démarre automatiquement après création.
- **Tags** : à choisir dans la liste curée par l'administrateur.

L'étape **Récapitulatif** résume tout avant validation. Quand vous atteignez
votre quota (VM max) ou une limite de gabarit, la demande est refusée avant
tout appel à Proxmox.

## Se connecter à sa VM

PVMSS n'ajoute ni clé SSH ni mot de passe à votre VM.

- **Clone de template** - connectez-vous avec les accès que votre
  administrateur a intégrés au template (une clé SSH ou un mot de passe).
  Toutes les VM clonées du même template les partagent ; demandez-les à votre
  administrateur si vous ne les avez pas.
- **Installation ISO** - installez le système depuis la console du navigateur
  et créez votre propre compte pendant l'installation.

Une fois la VM démarrée et son adresse signalée par l'agent invité, l'onglet
**Connexion** affiche la commande SSH. La console du navigateur fonctionne
dans tous les cas.

## Retrouver une machine virtuelle

**Mes VM** liste vos machines avec VMID, nom, cluster, nœud, tags, statut et
actions rapides (console, détails). Recherche, filtres et tri sont conservés
dans l'URL pour mettre une vue en favori ou la partager. Sélectionnez
plusieurs lignes pour lancer une **action d'alimentation groupée** ; chaque
VM rapporte son propre succès ou sa propre erreur.

Quand PVMSS est connecté à plusieurs environnements Proxmox, la liste
couvre le cluster auquel vous êtes connecté ; connectez-vous à un autre
cluster pour voir ses VM. Une VM est toujours identifiée par son `cluster` et son `VMID` : le
même VMID peut exister sur des clusters différents sans conflit.

## Gérer une machine virtuelle

La page de détail d'une VM comporte trois onglets : **Connexion** (actions d'alimentation et console), **Configuration** (avec les sous-sections ci-dessous : Résumé, Disques, Réseau, Matériel, Snapshots) et **Activité**.

### Connexion

- **Démarrer**, **Éteindre** (arrêt propre, agent invité / ACPI), **Redémarrer**, **Forcer l'arrêt** (coupure forcée), **Réinitialiser**, **Mettre en pause**, **Reprendre**.
- **Console** - ouvre la console graphique.
- **Démarrer sur le CD-ROM** une fois - redémarre sur l'ISO montée pour un seul démarrage.
- **Renommer** et modifier la **description** (le Markdown est rendu).
- **Supprimer** - supprime définitivement la VM (boîte de confirmation).
- **Métriques** - historique CPU, mémoire, disque et réseau sur la dernière heure, le dernier jour ou la dernière semaine.

Préférez **Éteindre** à **Forcer l'arrêt**. Si l'extinction ne fait rien, l'agent
QEMU manque probablement dans la VM : installez-le, ou utilisez **Forcer l'arrêt**.

### Disques

Ajoutez un disque sur un stockage approuvé, agrandissez un disque existant ou
détachez-en un. Les emplacements de bus sont limités par VM.

### Réseau

Modifiez chaque carte réseau : bridge, modèle, tag VLAN et limite de débit
(Mbit/s).

### Matériel

Modifiez sockets, cœurs et mémoire dans les limites de la politique, choisissez
les tags dans le sélecteur curé, et chargez ou éjectez une ISO dans le lecteur
CD-ROM.

### Snapshots

- **Créer** : saisissez un nom (commence par une lettre, puis lettres, chiffres, tirets ou underscores - 2 à 40 caractères), une description optionnelle, et choisissez d'inclure ou non l'état de la RAM.
- **Consulter** : nom, description, date de création et présence de la RAM ; l'état courant est marqué.
- **Restaurer** : ramène la VM à l'état du snapshot (la VM est arrêtée puis redémarrée). Opération destructive - les changements postérieurs sont perdus.
- **Supprimer** : retire définitivement un snapshot et libère son stockage.

Votre administrateur peut fixer un nombre maximal de snapshots par VM. Les
snapshots consomment du stockage : supprimez les anciens devenus inutiles.

### Activité

Chaque action effectuée sur la VM via PVMSS - qui, quoi, quand.

## Consoles

La page console propose deux clients :

- **Graphique** (noVNC) - l'affichage graphique, avec les mêmes actions d'alimentation que la page de détail.
- **Texte** (terminal série, xterm.js) pour les invités dotés d'un port série ; vous pouvez activer un port série sur une VM qui n'en a pas.

Les deux sont relayés par PVMSS avec un ticket à usage unique ; aucun accès
direct à Proxmox n'est nécessaire.

## Limites

Votre administrateur contrôle la plupart des limites par cluster ; PVMSS les
applique côté serveur avant tout appel à Proxmox.

- **Quota** - nombre maximal de VM par utilisateur.
- **Gabarit** - plafonds par VM : sockets, cœurs, mémoire, taille de disque, cartes réseau et snapshots.
- **Nom de VM** - un nom d'hôte en minuscules, 63 caractères maximum, unique dans votre pool.
- **Description** - 512 caractères maximum.
- **Actions groupées** - 100 VM maximum par requête.
- **Nom de snapshot** - une lettre puis lettres, chiffres, tirets ou underscores, 2 à 40 caractères ; `current` est réservé.

## Bonnes pratiques

- Utilisez des noms de VM descriptifs, avec tirets.
- Partez d'un profil quand l'un d'eux correspond à votre besoin.
- Ne conservez que les snapshots qui marquent une étape significative.

## Limitations connues

- Seules les VM KVM/QEMU sont prises en charge ; pas les conteneurs LXC.
- Les sauvegardes se font dans Proxmox, pas dans PVMSS. Seuls les administrateurs peuvent migrer une VM.
- Le réseau avancé (règles de pare-feu, SDN) se configure dans Proxmox.
- Le changement de mot de passe n'est disponible que par l'API pour l'instant.

## Sécurité et confidentialité

- Les sessions console sont authentifiées et liées à la session.
- Chaque utilisateur ne voit et ne gère que les VM de son propre pool ; c'est vérifié côté serveur à chaque requête.
- L'accès administrateur est séparé et exige une authentification supplémentaire.

## Astuces

- Utilisez la page de recherche pour des actions rapides sans ouvrir le détail.
- Mettez en favori des listes filtrées et des pages de détail de VM.
- L'application suit la langue de votre navigateur (français ou anglais).
