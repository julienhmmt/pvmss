# Règles de création de VM

Cette page rassemble les règles et recommandations qui encadrent la création
de VM dans PVMSS. Votre administrateur peut imposer des limites plus strictes
via la politique ; les valeurs indiquées ici sont celles par défaut.

## Nommage

Les noms de VM sont des noms d'hôte : minuscules, tirets, 63 caractères au
plus, uniques dans votre pool (le nom devient le libellé DNS de l'invité via
cloud-init). Évitez les noms génériques comme `vm1` - un nom descriptif
(`web-prod-01`) rend la liste des VM cherchable et le journal d'activité
lisible.

## Sources

- **ISO** - installation depuis une image approuvée par l'administrateur ; la
  VM démarre d'abord sur le CD-ROM.
- **Template** - clone d'un template Proxmox approuvé. La VM reste sur le nœud
  du template, le disque ne peut pas être plus petit que celui du template, et
  l'assistant vous prévient quand le stockage cible impose une copie complète
  au lieu d'un clone lié.
- **Image cloud** - importe une image cloud approuvée comme disque principal
  et exige les champs cloud-init (utilisateur, clés SSH, réseau). La VM ne
  démarre qu'une fois l'import terminé et cloud-init appliqué.

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

Les nouvelles VM démarrent en **UEFI** par défaut. Le **Secure Boot** est
désactivé par défaut : la plupart des ISO Linux embarquent un chargeur non
signé et s'installeraient sans jamais pouvoir démarrer depuis le disque ;
activez-le pour Windows. Activez le **TPM 2.0** pour les invités qui l'exigent
(Windows 11).

## Cloud-init

Préférez un **document cloud-init** à une configuration manuelle après
installation. Choisissez un template admin ou l'un de vos fichiers ; la VM en
reçoit sa propre copie à la création. Voir le [guide cloud-init](/docs/cloud-init-howto).

## Après la création

Les nouvelles VM apparaissent dans **Mes VM** dès la fin de la tâche de
création. Le premier démarrage peut prendre une minute pendant que cloud-init
s'exécute ; la console affiche la sortie de démarrage en direct.
