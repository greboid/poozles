const root = document.getElementById('input')
if (root) {
  document.getElementById('input').onsubmit = async (event) => {
    event.preventDefault()
    const formData= new FormData(event.target)
    const response = await fetch('/guess', {
      method: 'POST',
      body: formData
    }).then(res => res.json())
    if (response.result === 'correct') {
      alert('yay')
    } else if (response.result === 'incorrect') {
      alert('boo')
    } else if (response.result === 'unlock') {
      alert('unlock: ' + response.unlock)
    } else {
      alert('wtf')
      console.log(response)
    }
  }
}
