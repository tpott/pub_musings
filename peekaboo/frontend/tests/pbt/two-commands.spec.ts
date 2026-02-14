import { test } from '@playwright/test';
import { createPBT } from '../helpers/pbt/runner';

test('two sequential voice commands', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.say('show me a cat');
  await pbt.assertMedia('cat');

  await pbt.clear();
  await pbt.say('show me a dog');
  await pbt.assertMedia('dog');
});
