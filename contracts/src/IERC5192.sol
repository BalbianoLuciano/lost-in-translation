// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title ERC-5192: el estándar mínimo para decir "esto no se mueve".
/// @notice Es una sola función y dos eventos. Lo copiamos acá porque OpenZeppelin
///         todavía no lo trae: es más honesto tener la interfaz a la vista que
///         inventarse un `bool transferible` que nadie afuera sabe leer.
/// @dev El identificador ERC-165 de esta interfaz es `0xb45a3c0e`.
interface IERC5192 {
    /// @notice Se emite cuando el token queda trabado. Acá, siempre al acuñar.
    event Locked(uint256 tokenId);

    /// @notice Se emite cuando el token se destraba. Acá no se emite nunca.
    event Unlocked(uint256 tokenId);

    /// @notice Devuelve si el token está trabado.
    function locked(uint256 tokenId) external view returns (bool);
}
