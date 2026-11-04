# Règles de création de VM

Les valeurs de cette page sont celles du portail par défaut. Votre
administrateur peut imposer des limites plus strictes.

## Nommage

Les noms de VM sont des noms d'hôte : minuscules, tirets, 63 caractères au
plus, uniques dans votre pool. Un nom descriptif comme `web-prod-01` se retrouve plus facilement que `vm1`
dans la liste des VM et le journal d'activité.

## Sources

- **ISO** - installation depuis une image approuvée par l'administrateur ; la
  VM démarre sur l'installeur tant que le disque est vide, puis sur son disque
  une fois installée, même si l'ISO reste montée.
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

Le champ **OS** (installations depuis ISO uniquement) définit la famille du
système. Choisir **Windows 11** active l'UEFI et le TPM 2.0, que l'installation
de Windows 11 exige tous deux. Une installation Windows reçoit un disque SATA
et, sauf si vous choisissez un autre modèle, une carte réseau e1000, car son
programme d'installation n'a pas de pilotes VirtIO.

## Après la création

Les nouvelles VM apparaissent dans **Mes VM** dès la fin de la tâche de
création. Le premier démarrage peut prendre une minute ; la console affiche
la sortie de démarrage en direct.
