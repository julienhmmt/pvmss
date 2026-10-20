# Règles de création de VM

Cette page rassemble les règles et recommandations qui encadrent la création
de VM dans PVMSS. Votre administrateur peut imposer des limites plus strictes
via la politique ; les valeurs indiquées ici sont celles par défaut.

## Nommage

Les noms de VM sont des noms d'hôte : minuscules, tirets, 63 caractères au
plus, uniques dans votre pool. Évitez les noms génériques comme `vm1` - un nom descriptif
(`web-prod-01`) rend la liste des VM cherchable et le journal d'activité
lisible.

## Sources

- **ISO** - installation depuis une image approuvée par l'administrateur ; la
  VM démarre d'abord sur le CD-ROM.
- **Template** - clone d'un template Proxmox approuvé. La VM reste sur le nœud
  du template, le disque ne peut pas être plus petit que celui du template, et
  l'assistant vous prévient quand le stockage cible impose une copie complète
  au lieu d'un clone lié. Le clone garde les accès intégrés au template (clé
  SSH ou mot de passe) : PVMSS n'en ajoute aucun, demandez-les à votre
  administrateur.

## Ressources

- Partez d'un **profil** quand l'un d'eux correspond à votre besoin ; les
  profils encodent les combinaisons CPU/mémoire/disque approuvées.
- Les valeurs personnalisées sont bornées par la politique du cluster : une
  demande au-delà du quota utilisateur, du gabarit ou de la capacité du nœud
  est refusée avant tout appel à Proxmox.
- Laissez le nœud vide et PVMSS choisit le nœud approuvé le moins chargé avec
  assez d'espace de stockage.
- Les disques utilisent le stockage sélectionné ; choisissez-le en fonction du
  profil d'E/S attendu.

## Firmware

Les nouvelles VM démarrent en **UEFI** par défaut, avec un disque EFI dont le
magasin de clés est **vide**. Autrement dit : UEFI (GPT, variables EFI, type
de machine q35) mais **pas de Secure Boot** - PVMSS ne l'active jamais, par
choix. Secure Boot n'exécute que les chargeurs signés par les clés inscrites
dans les variables EFI, et PVMSS crée ses VM à partir de l'ISO approuvée par
l'administrateur : la plupart des médias d'installation Linux ne sont pas
signés (l'image officielle d'Arch indique elle-même ne pas prendre en charge
Secure Boot), donc avec Secure Boot activé l'installeur ne démarre jamais et
la VM s'arrête sur le shell UEFI, sans retour possible. Si vous avez besoin de
Secure Boot - pour un invité Windows 11 par exemple - activez-le dans Proxmox,
où vous pouvez vérifier que l'ISO concernée est bien signée.

Activez le **TPM 2.0** pour les invités qui l'exigent (Windows 11).

## Après la création

Les nouvelles VM apparaissent dans **Mes VM** dès la fin de la tâche de
création. Le premier démarrage peut prendre une minute ; la console affiche
la sortie de démarrage en direct.
