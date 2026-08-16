# Guide de l'utilisateur

PVMSS (Proxmox Virtual Machine Self-Service) est un portail en libre-service
qui vous permet de créer, gérer et accéder aux consoles des machines
virtuelles hébergées sur un serveur Proxmox Virtual Environment, sans utiliser
directement l'interface Proxmox.

## Démarrage rapide

1. **Connectez-vous** sur la [page de connexion](/login) avec les identifiants fournis par votre administrateur.
2. **Trouvez vos VMs** depuis la page Mes VMs ; recherchez par nom ou VMID.
3. **Créez une VM** avec le bouton « Créer une VM », puis renseignez les paramètres requis.
4. **Ouvrez la console** une fois la VM créée et démarrée, via le client noVNC intégré.
5. **Gérez votre profil** pour voir vos VMs et changer votre mot de passe.

## Créer une machine virtuelle

Ouvrez le formulaire de configuration via « Créer une VM » après vous être connecté. Configurez :

- **Nœud** : le nœud Proxmox où la VM sera créée, parmi ceux approuvés par votre administrateur. Un nœud peut être désactivé s'il a atteint les limites configurées.
- **Nom et description** : un nom minuscule, par tirets, unique dans votre pool. Un nom clair (comme `web-prod-01`) rend la liste recherchable et le journal d'audit lisible.
- **Système d'exploitation** : une image ISO de la liste approuvée par l'administrateur.
- **Profil (optionnel)** : si votre administrateur a publié des profils matériels, choisissez-en un pour remplir CPU, mémoire, disque et bus automatiquement.
- **Ressources** : CPU (sockets et cœurs), mémoire (Mo ou Go), et disques. Les valeurs sont bornées par la politique du cluster et votre quota par utilisateur.
- **Stockage** : un stockage approuvé par votre administrateur.
- **Réseau** : une ou plusieurs cartes réseau. Pour chaque carte, vous pouvez choisir le pont (VMBR), le modèle (VirtIO, E1000, E1000E, RTL8139, VMXNet3), une adresse MAC optionnelle, un tag VLAN optionnel (1-4096), et une vitesse réseau optionnelle (Mo/s).
- **Firmware et sécurité** : démarrage EFI (UEFI) et TPM v2.0 optionnel pour les invités qui l'exigent (par exemple Windows 11).
- **Cloud-init** : choisissez un modèle géré par l'administrateur, ou laissez pour plus tard.
- **Démarrage** : choisissez si la VM démarre automatiquement après sa création.
- **Tags** : ajoutez des tags prédéfinis pour organiser vos VMs.

Vous pouvez créer une VM à la fois. Lorsque vous atteignez votre quota (VM max, CPU, mémoire ou disque), la requête est rejetée avant tout appel Proxmox.

## Trouver une machine virtuelle

Utilisez la recherche pour localiser une VM par nom, VMID ou tag. Les résultats affichent le VMID, le nom, le nœud hôte, les tags (hors tag interne `pvmss`), l'état, et un bouton pour ouvrir les détails.

## Gérer une machine virtuelle

La page de détails de la VM vous donne le contrôle complet :

- **Démarrer** — allumer la VM.
- **Console** — ouvrir la console noVNC intégrée dans une nouvelle fenêtre.
- **Redémarrer** — redémarrer la VM.
- **Éteindre** — arrêt ACPI gracieux.
- **Arrêter** — arrêt forcé (extinction immédiate).
- **Reset** — réinitialisation forcée.
- **Actualiser** — rafraîchir les informations de la VM (invalidation du cache).
- **Supprimer** — supprimer définitivement la VM (confirmation requise).

Préférez **Éteindre** (gracieux) à **Arrêter** (forcé). Si vous voyez des messages répétés indiquant que l'agent QEMU guest est indisponible, installez ou activez l'agent dans la VM, ou utilisez **Arrêter**.

### Modifier les ressources

Lorsqu'une VM est **arrêtée**, vous pouvez modifier certaines ressources depuis la page de détails :

- CPU (sockets et cœurs), dans les limites de la politique.
- Mémoire (Mo/Go), dans les limites de la politique.
- Cartes réseau (pont, modèle, MAC optionnelle).
- Snippet cloud-init (un `#cloud-config` personnalisé).
- CD-ROM / ISO (charger ou éjecter une ISO).

L'augmentation de la taille des disques et autres changements structurels au-delà de la politique doivent se faire dans Proxmox.

## Cloud-init

Cloud-init configure une VM au premier démarrage sans connexion : utilisateurs, clés SSH, paquets, et plus.

- Sur **Créer une VM**, choisissez un modèle géré par l'administrateur dans le menu cloud-init ; son contenu est appliqué à l'identique.
- Après création, ouvrez l'onglet **Cloud-init** de la VM pour voir ou éditer le snippet. L'éditeur accepte tout document `#cloud-config` valide. Les changements sont poussés vers le cluster et prennent effet au prochain démarrage.

Les champs pris en charge incluent `packages`, `users`, `write_files`, et `runcmd`. Voir la [documentation cloud-init](https://cloudinit.readthedocs.io/) pour le schéma complet.

## Snapshots

Les snapshots sauvegardent l'état complet d'une VM à un instant donné et le restaurent plus tard.

- **Créer** : ouvrez la page de détails de la VM, allez dans la section snapshots, saisissez un nom (alphanumérique, tirets, underscores, 40 caractères max) et une description optionnelle, incluez éventuellement l'état RAM, puis cliquez sur Créer.
- **Voir** : la liste affiche le nom, la description, la date de création, et l'état (avec RAM ou disque seulement). L'état courant est marqué d'une étoile.
- **Éditer la description** : utilisez le bouton crayon sur une ligne de snapshot.
- **Restaurer** : ramène la VM à l'état du snapshot. C'est destructif — les changements après le snapshot sont perdus.
- **Supprimer** : supprime définitivement un snapshot et libère son stockage.

Votre administrateur peut définir un nombre maximal de snapshots par VM. Les snapshots consomment du stockage, supprimez donc les anciens quand ils ne sont plus utiles.

## Profil et mot de passe

Votre page **Profil** résume vos VMs (total, en cours, arrêtées) et propose un formulaire sécurisé pour changer votre mot de passe. La page **Jetons API** permet de créer des jetons d'accès personnels pour le scripting, si votre administrateur les a activés.

## Bonnes pratiques

- Utilisez des noms de VM descriptifs, par tirets.
- Préférez un modèle cloud-init à une configuration post-installation manuelle.
- Partez d'un profil quand il correspond à votre charge de travail.
- Réservez les snapshots à des points de contrôle significatifs.

## Limites connues

- La reconfiguration des ressources est limitée à CPU, mémoire, cartes réseau et ISO pour une VM arrêtée. Agrandir les disques ou changer leur nombre au-delà de la politique nécessite Proxmox.
- Seules les VMs KVM/QEMU sont prises en charge ; les conteneurs LXC ne le sont pas.
- Les sauvegardes et la migration à chaud sont gérées dans Proxmox, pas dans PVMSS.
- Le réseau avancé (règles de pare-feu, SDN) est configuré dans Proxmox.

## Sécurité et confidentialité

- Les sessions console sont authentifiées et basées sur les sessions.
- Chaque utilisateur ne peut voir et gérer que les VMs de son propre pool.
- L'accès administrateur est séparé et nécessite une authentification supplémentaire.

## Astuces

- Utilisez la page de recherche pour des actions démarrer/arrêter rapides sans ouvrir les détails.
- Ouvrez plusieurs fenêtres de console pour gérer plusieurs VMs à la fois.
- Mettez en favori l'URL du portail et les pages de détails de VMs spécifiques.
- L'application suit la préférence de langue de votre navigateur (anglais ou français).
