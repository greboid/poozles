WIP

 - Create a puzzles directory
 - Create a puzzles/index.html file which contains the body of the index page
 - Create folders in the puzzles directory for each puzzle

Each puzzle should contain an index.html and can contain any number of files
This needs to have some frontmatter eg
```
<!--
title: Example puzzle title
answers: ["melisma"]
hints: ["it's not a real word"]
-->
```
After this include the html content of the puzzle, linking to any of the files in the folder

A guess box is added automatically and guesses submitted are handled and display the result with alert()
