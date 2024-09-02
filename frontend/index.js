const URL = "http://localhost:8080/v1/validate"
//const URL = "https://us-central1-yaml2go.cloudfunctions.net/yaml2go"

let editor = ""

window.generatorCall=function (){
  let yamlData  = document.getElementById("yamlSpecs").value
  document.getElementById('yamlSpecs').style.border = "1px solid #ced4da"
  yamlData = editor.getValue()
  $.ajax({
    'url' : `${URL}`,
    'type' : 'POST',
    'data' : yamlData,
    'success' : function(data) { 
        document.getElementById("message").style.display="block";
        document.getElementById("message").style.color = "green"
        document.getElementById("message-span").innerHTML="Blueprint is Valid!";
        const container = document.getElementById("sequence");
        container.innerHTML=data;
        container.removeAttribute("data-processed");
        mermaid.init(undefined, container);
    },
    'error' : function(jqXHR, request, error)
    {
      document.getElementById('yamlSpecs').style.border = "1px solid red"
      if (jqXHR.status == 400) {
        displayError(jqXHR.responseText)
      } else {
        displayError('Something went wrong! Please report this to me@prasadg.dev')
      }
    }
  });

}

// Validate
document.getElementById("validate").addEventListener('click', ()=>{
   generatorCall()
})

// Clear YAML
document.getElementById('clearYaml').addEventListener('click',()=>{
  editor.setValue('')
})

$(document).ready(function(){
    //code here...
    var input = $(".codemirror-textarea")[0];
    editor = CodeMirror.fromTextArea(input, {
        mode: "text/x-yaml",
    	lineNumbers : true
    });
    mermaid.initialize({ startOnLoad: false });
    editor.setValue('# Paste Blueprint specs yaml here...\n')
});

function displayError(err){
  document.getElementById("message-span").innerHTML=err;
  document.getElementById("message").style.display="block"
  document.getElementById("message").style.color = "red"
}