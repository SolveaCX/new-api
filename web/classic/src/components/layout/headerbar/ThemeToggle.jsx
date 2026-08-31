/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Button } from '@douyinfe/semi-ui';
import { Sun } from 'lucide-react';

const ThemeToggle = ({ t }) => {
  return (
    <span className='inline-flex' title={t('浅色模式')}>
      <Button
        icon={<Sun size={18} />}
        aria-label={t('浅色模式')}
        theme='borderless'
        type='tertiary'
        className='!p-1.5 !text-current !rounded-full !bg-semi-color-fill-0'
      />
    </span>
  );
};

export default ThemeToggle;
