// Gestion simplifiée des limites de ressources
document.addEventListener('DOMContentLoaded', () => {
  // Fonction utilitaire pour récupérer le jeton CSRF
  const getCSRFToken = () => {
    const meta = document.querySelector('meta[name="csrf-token"]')
    return meta ? meta.getAttribute('content') : ''
  }

  // Gestion des formulaires de limites
  document.querySelectorAll('form.limits-form').forEach((form) => {
    // Fixe la traduction dynamique du bouton 'Enregistrer'
    const saveBtn = form.querySelector('.save-limits-btn')
    if (saveBtn) {
      saveBtn.textContent = window.I18N_SAVE || 'Save'
    }

    form.addEventListener('submit', async (e) => {
      e.preventDefault()
      const { entityId } = form.dataset
      const statusEl = form.querySelector('.status-indicator')
      if (statusEl) {
        statusEl.textContent = window.I18N_SAVING || 'Saving...'
        statusEl.className = 'status-indicator has-text-info'
      }

      // Construction du payload JSON (seulement les max, min toujours 1 côté backend)
      const payload = { entityId }
      const fields = ['sockets', 'cores', 'ram', 'disk']
      fields.forEach((field) => {
        const maxInput = form.querySelector(`[name="${field}-max"]`)
        if (maxInput && maxInput.value !== '') {
          payload[field] = { min: 1, max: parseInt(maxInput.value, 10) }
        }
      })

      try {
        const headers = {
          'Content-Type': 'application/json',
        }
        
        // Utiliser la fonction utilitaire getCSRFToken pour récupérer le jeton
        const csrfToken = getCSRFToken()
        if (csrfToken) {
          headers['X-CSRF-Token'] = csrfToken
        }
        
        const response = await fetch(form.action, {
          method: 'POST',
          headers,
          body: JSON.stringify(payload),
        })

        // Vérifier le type de contenu de la réponse
        const contentType = response.headers.get('content-type') || ''
        console.log('Content-Type de la réponse:', contentType)

        // Lire le contenu de la réponse
        const text = await response.text()
        console.log('Réponse brute du serveur:', text)

        let result
        try {
          // Essayer de parser la réponse en JSON
          result = JSON.parse(text)
          console.log('Réponse parsée avec succès:', result)
        } catch (error) {
          console.error('Erreur de parsing JSON:', error)
          console.error('Contenu de la réponse qui a échoué au parsing:', text)

          // Afficher plus d'informations sur la réponse
          console.error(
            'Status de la réponse:',
            response.status,
            response.statusText
          )
          console.error('En-têtes de la réponse:')
          response.headers.forEach((value, key) => {
            console.error(`  ${key}: ${value}`)
          })

          throw new Error('La réponse du serveur n\'est pas un JSON valide')
        }

        if (statusEl) {
          if (response.ok && result.success) {
            statusEl.textContent = window.I18N_SAVED || '✓ Saved'
            statusEl.className = 'status-indicator has-text-success'
          } else {
            statusEl.textContent = result.message || window.I18N_ERROR || 'Error'
            statusEl.className = 'status-indicator has-text-danger'
          }
          setTimeout(() => {
            if (
              statusEl.textContent.includes('Enregistré') ||
              statusEl.textContent.includes('Erreur')
            ) {
              statusEl.textContent = ''
              statusEl.className = 'status-indicator'
            }
          }, 3000)
        }

        if (result.redirect) {
          window.location.href = result.redirect
        }
      } catch (error) {
        console.error('Erreur:', error)
        if (statusEl) {
          statusEl.textContent = 'Erreur de connexion'
          statusEl.className = 'status-indicator has-text-danger'
        }
      }
    })
  })

  // Gestion des boutons de sauvegarde
  document.querySelectorAll('.save-limits-btn').forEach((button) => {
    button.addEventListener('click', (e) => {
      e.preventDefault()
      const form = button.closest('form')
      if (!form) return
      button.disabled = true
      const originalLabel = button.textContent
      button.textContent = window.I18N_SAVING || 'Enregistrement...'
      // On déclenche le submit JS du formulaire
      form.requestSubmit()
      setTimeout(() => {
        button.disabled = false
        button.textContent = originalLabel
      }, 2000)
    })
  })

  // Gestion des boutons de réinitialisation
  document.querySelectorAll('.reset-defaults-btn').forEach((button) => {
    button.addEventListener('click', async (e) => {
      e.preventDefault()
      if (!window.confirm('Réinitialiser les valeurs par défaut ?')) return

      const form = button.closest('form')
      const entityId = button.dataset.entityId || form?.dataset.entityId
      const statusEl = form?.querySelector('.status-indicator')
      if (statusEl) {
        statusEl.textContent = 'Réinitialisation...'
        statusEl.className = 'status-indicator'
      }
      try {
        const response = await fetch('/api/limits/reset', {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ entity_id: entityId }),
        })
        const result = await response.json()
        if (result.success && result.limits) {
          // Mettre à jour dynamiquement les champs du formulaire
          const { limits } = result
          const updateField = (name, val) => {
            const input = form.querySelector(`[name="${name}"]`)
            if (input) input.value = val
          }
          // VM ou node: mêmes champs
          if (limits.sockets) {
            updateField('sockets-min', limits.sockets.min)
            updateField('sockets-max', limits.sockets.max)
          }
          if (limits.cores) {
            updateField('cores-min', limits.cores.min)
            updateField('cores-max', limits.cores.max)
          }
          if (limits.ram) {
            updateField('ram-min', limits.ram.min)
            updateField('ram-max', limits.ram.max)
          }
          if (limits.disk) {
            updateField('disk-min', limits.disk.min)
            updateField('disk-max', limits.disk.max)
          }
          if (statusEl) {
            statusEl.textContent = 'Valeurs par défaut restaurées'
            statusEl.className = 'status-indicator has-text-success'
          }
        } else if (statusEl) {
          statusEl.textContent = result.message || 'Erreur de réinitialisation'
          statusEl.className = 'status-indicator has-text-danger'
        }
      } catch (error) {
        console.error('Erreur:', error)
        if (statusEl) {
          statusEl.textContent = 'Erreur de connexion'
          statusEl.className = 'status-indicator has-text-danger'
        }
      }
      setTimeout(() => {
        if (statusEl) statusEl.textContent = ''
      }, 3000)
    })
  })
})
