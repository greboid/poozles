const root = document.getElementById('input')
if (root) {
  const unlocks = document.getElementById('unlocks')
  const guesses = document.getElementById('guesses')
  root.onsubmit = async (event) => {
    event.preventDefault()
    const now = new Date()
    const formData= new FormData(event.target)
    const response = await fetch('/guess', {
      method: 'POST',
      body: formData
    }).then(res => res.json())
    root.elements.guess.value = ""
    if (guesses.innerHTML.length > 0) {
      guesses.innerHTML += "<br>"
    }
    guesses.innerText += now.getHours().toString().padStart(2, '0') + ":" + now.getMinutes() + " - " + response.guess
    if (response.result === 'correct') {
      root.classList.remove('error')
      root.classList.add('correct')
    } else if (response.result === 'incorrect') {
      root.classList.remove('correct')
      root.classList.add('error')
    } else if (response.result === 'unlock') {
      if (unlocks.innerHTML.length > 0) {
        unlocks.innerHTML += "<br>"
      }
      guesses.innerText += " [Unlock]"
      unlocks.innerText += now.getHours().toString().padStart(2, '0') + ":" + now.getMinutes().toString().padStart(2, '0') + " - " + response.guess + " => " + response.unlock
    } else {
      alert('wtf')
      console.log(response)
    }
  }
  root.oninput = (event) => {
    root.classList.remove('error')
    root.classList.remove('correct')
  }

  document.querySelectorAll('.hint button').forEach((el) =>
      el.addEventListener('click', (event) => {
        const parent = event.target.closest('.hint')
        const hintId = parseInt(parent.dataset.index)
        const puzzle = root.elements.puzzle.value
    
        fetch('/hint', {
          method: 'POST',
          body: JSON.stringify({puzzle: puzzle, hintRequested: hintId})
        })
            .then(res => res.json())
            .then(response => {
              parent.querySelector('p').innerText = response.hint
              parent.querySelector('p').className = 'unlocked'
              event.target.remove()
            })
      })
  )
}
