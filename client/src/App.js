import React, { useState } from "react";

import "./App.css";

import axios from "axios";

import MonacoEditor from "react-monaco-editor";

const languages = [
  {
    title: "Select language",
    name: "",
    syntax: "",
  },
  {
    title: "Node",
    name: "node",
    syntax: "javascript",
  },
  {
    title: "Python",
    name: "python",
    syntax: "python",
  },
  {
    title: "C Programming",
    name: "c",
    syntax: "cpp",
  },
  {
    title: "C++ Programming",
    name: "cpp",
    syntax: "cpp",
  },
  {
    title: "Golang",
    name: "go",
    syntax: "go",
  },
];

function App() {
  const [code, setCode] = useState("");
  const [result, setResult] = useState("");
  const [language, setLanguage] = useState(languages[0]);
  const [loading, setLoading] = useState(false);

  const options = {
    selectOnLineNumbers: true,
    fontSize: 16,
  };

  return (
    <div className="app">
      <div className="header">
        <div className="brand">
          <h1 className="title is-3">Remote Code</h1>
        </div>
        <button
          className={`button is-success ${loading && "is-loading"}`}
          onClick={execute}
          disabled={language.name.length === 0 && language.syntax.length === 0}
        >
          Execute
        </button>
        <div className="language select">
          <select onChange={onLanguageChange}>
            {languages.map((language) => (
              <option key={language.name} value={language.name}>
                {language.title}
              </option>
            ))}
          </select>
        </div>
      </div>
      <div className="editor-container">
        <div className="editor">
          <MonacoEditor
            language={language.syntax}
            options={options}
            theme="vs-dark"
            value={code}
            onChange={setCode}
          />
        </div>
        <div className="result">
          <h1 className="result-title title is-4">Output</h1>
          <p className="result-value">{result}</p>
        </div>
      </div>
    </div>
  );

  function execute() {
    setResult("");
    setLoading(true);
    axios
      .post("http://35.175.133.115:8080/execute", {
        type: language.name,
        code,
      })
      .then((data) => {
        setResult(data.data.response);
        setLoading(false);
      })
      .catch((error) => {
        setResult(error.response.data.message);
        setLoading(false);
      });
  }

  function onLanguageChange(event) {
    const value = event.target.value;
    const language = languages.find((language) => language.name === value);
    setLanguage(language);
  }
}

export default App;
