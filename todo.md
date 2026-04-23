# PVMSS Todo and fixes

## Todo

- [x] **TO CHECK** frontend : avoir un squelette mais sans les donner, pour afficher ce qui va arriver une fois les données récupérées
- [ ] admin/nodes : il faut un bouton à côté de chaque node pour activer/désactiver le node qu'on laisse à l'utilisateur. On peut déjà le faire dans les paramètres avancés en tant qu'administrateur mais il faut un bouton plus facile d'accès pour les admin dans la page admin/nodes
- [x] **TO CHECK** admin/limites : avoir un bouton à chaque node pour 'appliquer ces limites à tous les nodes'
- [x] **TO CHECK** admin/iso : permettre de filtrer par node
- [x] **TO CHECK** frontend : la largeur et l'affichage de la page d'accueil est bonne, il faut avoir cette même disposition dans toutes les autres pages (home, vm/create (et toutes les pages sous-jacentes), search, documentation, profile)
- [ ] user/createVM : dans les profils il faut ajouter l'efi (dire que c'est le démarrage sécurisé), le cocher par défaut et mettre sur un stockage qui supporte l'efi (il existe déjà ce check)

## Fixes

- [ ] admin/pool utilisateurs : erreur dans le browser 'uncaught typeError: cannot read properties of undefined (reading 'slice') at 23.pGpztlGH.js:2:1544 / at Array.map (<anonymous>) DTeh809U.js
- [ ] admin/cloud-init : mettre un bouton pour activer ou désactiver le sftp
- [ ] admin/cloud-init : avoir un statut (config)
- [ ] user/createVM : error 400 en sélectionnant un profil : on a pas le choix du réseau donc ça plante
- [ ] user/createVM : error 500 post <https://pvmss-beta.domain.local/api/v1/vms> en voulant créer une vm en mode 'avancé' : POST request returned error status 400 for /nodes/pve1/qemu message: parameter verification failed: errors: ide2 invalid format - format error. ide2.file: unable to parse volume ID
- [ ] admin/doc : la doc est par défaut en anglais, il faut qu'elle suive la langue que l'utilisateur a choisi (fr ou en)
- [ ] admin/nodes : le nombre de coeur ne s'affiche pas, il y a juste 'coeurs'
- [ ] user/createVM : la liste déroulante pour les stockages n'est pas alignée avec la liste déroulante du node
- [x] **TO CHECK** user/createVM : dans le bus disque c'est en anglais - traduction manquante (virtio block - recommended)
- [x] **TO CHECK** user/createVM : dans la configuration des disques il faut un compteur ou un tooltip pour afficher le nombre de disque actuel, la capacité max selon la limite imposée par les admin pvmss
- [x] **TO CHECK** user/createVM : dans la configuration des vmbr il faut un compteur ou un tooltip pour afficher le nombre de carte réseau actuel, la capacité max selon la limite imposée par les admin pvmss
- [ ] user/createVM : cloud-init si les admin pvmss n'ont pas activé l'option alors il ne faut pas proposer l'option dans le formulaire de création de vm
