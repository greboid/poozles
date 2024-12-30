# Setup

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
unlocks: {"you missed a letter!": ["melism", "elisma"]}
-->
```
After this include the html content of the puzzle, linking to any of the files in the folder.

If a success.html file is present in the puzzle folder this will replace the puzzle on a correct guess.

# Configuration

 - `PORT` - Web server port
 - `DEBUG` - Run in debug mode
 - `DB_TYPE` - Specifies what kind of database to use, currently supports noop
