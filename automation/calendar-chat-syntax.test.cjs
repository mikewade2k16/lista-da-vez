const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const workflow = require('./export/workflow-calendar-chat.json')[0]

test('código dos nós do Calendar Chat é JavaScript válido', () => {
  const codeNodes = workflow.nodes.filter((node) => node.parameters?.jsCode)
  assert.ok(codeNodes.length > 0)
  for (const node of codeNodes) {
    assert.doesNotThrow(
      () => new vm.Script(`(async () => {\n${node.parameters.jsCode}\n})()`),
      `Nó ${node.name}`,
    )
  }
})
