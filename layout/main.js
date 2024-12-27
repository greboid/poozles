const root = document.getElementById('input')
if (root) {
  const answerBox = root.querySelector('[name="guess"]')
  const hints = document.getElementById('hints')
  const unlocks = document.getElementById('unlocks')
  const guesses = document.getElementById('guesses')
  root.onsubmit = async (event) => {
    event.preventDefault()
    const now = new Date()
    const formData = new FormData(event.target)
    answerBox.value = '';
    const response = await fetch('/guess', {
      method: 'POST',
      body: formData
    }).then(res => res.json())
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
      unlocks.innerText += now.getHours().toString().padStart(2, '0') + ":" + now.getMinutes() + " - " + response.guess + " => " + response.unlock
    } else {
      alert('wtf')
      console.log(response)
    }
  }
  root.oninput = (event) => {
    root.classList.remove('error')
    root.classList.remove('correct')
  }
  hints.onclick = (event) => {
    if (event.target.tagName !== 'LI') {
      return
    }
    const hintRequested = [...event.target.parentNode.children].indexOf(event.target)
    const puzzle = root.elements.puzzle.value
    if (!confirm(`Are you sure you want to request hint ${hintRequested+1}?`)) {
      return
    }
    fetch('/hint', {
      method: 'POST',
      body: JSON.stringify({puzzle: puzzle, hintRequested: hintRequested})
    })
        .then(res => res.json())
        .then(response => {
          hints.children[response.hintRequested].innerText = response.hint
        })
  }
}
