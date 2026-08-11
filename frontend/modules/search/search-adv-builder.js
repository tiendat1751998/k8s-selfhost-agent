/**
 * Enterprise Search Advanced Rules Builder Engine
 */
(function (global) {
  'use strict';

  var builder = null;
  function getBuilder() {
    if (!builder && global.RulesBuilder) {
      builder = new global.RulesBuilder({
        fields: [
          { value: "message", label: "Message" },
          { value: "cluster", label: "Cluster" },
          { value: "namespace", label: "Namespace" },
          { value: "severity", label: "Severity" }
        ],
        operators: [
          { value: "contains", label: "Contains" },
          { value: "not_contains", label: "Does Not Contain" },
          { value: "equals", label: "Equals" },
          { value: "not_equals", label: "Does Not Equal" }
        ],
        evaluateFieldRule: function (item, field, op, val) {
          var fieldVal = '';
          if (field === 'message') {
            fieldVal = item.name;
          } else if (field === 'cluster') {
            fieldVal = item.cluster;
          } else if (field === 'namespace') {
            fieldVal = item.namespace;
          } else if (field === 'severity') {
            if (item.name.indexOf('ERROR') >= 0 || item.name.indexOf('restarted') >= 0 || item.name.indexOf('NotReady') >= 0) {
              fieldVal = 'ERROR';
            } else if (item.name.indexOf('WARN') >= 0) {
              fieldVal = 'WARN';
            } else {
              fieldVal = 'INFO';
            }
          }

          var fVal = (fieldVal || '').toLowerCase();
          var qVal = (val || '').toLowerCase();

          switch (op) {
            case 'contains':
              return fVal.indexOf(qVal) >= 0;
            case 'not_contains':
              return fVal.indexOf(qVal) < 0;
            case 'equals':
              return fVal === qVal;
            case 'not_equals':
              return fVal !== qVal;
            default:
              return false;
          }
        }
      });
    }
    return builder;
  }

  function createRuleElement() {
    return getBuilder().createRuleElement();
  }

  function createGroupElement() {
    return getBuilder().createGroupElement();
  }

  function compileRules(container) {
    return getBuilder().compileRules(container);
  }

  function evaluateCompiledRules(item, compiledRules, matchType) {
    return getBuilder().evaluateCompiledRules(item, compiledRules, matchType);
  }

  function loadRulesIntoDOM(rules, container) {
    getBuilder().loadRulesIntoDOM(rules, container);
  }

  global.SearchAdvBuilder = {
    createRuleElement: createRuleElement,
    createGroupElement: createGroupElement,
    compileRules: compileRules,
    evaluateCompiledRules: evaluateCompiledRules,
    loadRulesIntoDOM: loadRulesIntoDOM
  };

})(window);
