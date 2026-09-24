// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {Test} from "forge-std/Test.sol";
import {Strings} from "@openzeppelin/contracts/utils/Strings.sol";

import {Distinciones} from "../src/Distinciones.sol";

/// @dev Lo que comparten todas las pruebas: el catálogo de 41 y la firma del
///      voucher armada a mano. La firma se construye desde cero, sin usar nada
///      del contrato salvo el dominio que él mismo publica, para que un cambio
///      en el hash del tipo rompa las pruebas en vez de pasar desapercibido.
abstract contract Base is Test {
    bytes32 internal constant TIPO_DISTINCION =
        keccak256("Distincion(address to,uint16 pieza,uint64 deadline)");

    /// @dev 34 piezas + 7 obras, que es lo que sale de `content/skills.yaml`.
    uint16 internal constant CANTIDAD_PIEZAS = 41;

    /// @dev El orden del secp256k1. Una clave privada válida va de 1 a n-1.
    uint256 internal constant ORDEN_SECP256K1 =
        0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141;

    uint256 internal constant PK_FIRMANTE = 0xA11CE;
    uint256 internal constant PK_INTRUSO = 0xB0B;

    Distinciones internal distinciones;
    address internal duenio = makeAddr("duenio");
    address internal firmante;
    address internal titular = makeAddr("titular");

    function setUp() public virtual {
        firmante = vm.addr(PK_FIRMANTE);
        distinciones = new Distinciones(duenio, firmante, _catalogo());
    }

    /// @dev Las 34 primeras son piezas de las 7 obras; las 7 últimas, las obras.
    ///
    ///      Las juntas se arman encadenadas, como en el catálogo de verdad: la
    ///      de abajo de una pieza es la de arriba de la siguiente de su obra, y
    ///      las dos puntas del pilar van rectas. El perfil de cada empalme sale
    ///      de `(obra * 3 + local) % 10` sin más motivo que barrer los diez
    ///      perfiles entre las siete obras: si uno estuviera roto, algún dibujo
    ///      de las pruebas lo mostraría.
    function _catalogo() internal pure returns (Distinciones.Pieza[] memory catalogo) {
        catalogo = new Distinciones.Pieza[](CANTIDAD_PIEZAS);
        for (uint8 i = 0; i < 34; ++i) {
            uint8 obra = i % 7;
            uint8 local = i / 7;
            // 34 = 7*4 + 6: a las seis primeras obras les tocan cinco piezas.
            uint8 cuantas = obra < 6 ? 5 : 4;

            catalogo[i] = Distinciones.Pieza({
                nombre: string.concat("Pieza ", Strings.toString(i)),
                obra: obra,
                juntaArriba: local == 0 ? 0 : _empalme(obra, local - 1),
                juntaAbajo: local == cuantas - 1 ? 0 : _empalme(obra, local),
                esObra: false
            });
        }
        for (uint8 i = 0; i < 7; ++i) {
            catalogo[34 + i] = Distinciones.Pieza({
                nombre: string.concat("Obra ", Strings.toString(i)),
                obra: i,
                juntaArriba: 0,
                juntaAbajo: 0,
                esObra: true
            });
        }
    }

    /// @dev El perfil del empalme entre la pieza `local` de la obra y la que sigue.
    function _empalme(uint8 obra, uint8 local) internal pure returns (uint8) {
        return (obra * 3 + local) % 10;
    }

    /// @dev El separador de dominio se lee del propio contrato vía ERC-5267 y se
    ///      vuelve a armar acá: si el `name`, la `version`, la cadena o la
    ///      dirección no son los que el diseño pide, la prueba lo grita.
    function _separadorDominio() internal view returns (bytes32) {
        (
            ,
            string memory name,
            string memory version,
            uint256 chainId,
            address verifyingContract,,
        ) = distinciones.eip712Domain();

        assertEq(name, "Lost in Translation", "el nombre del dominio cambio");
        assertEq(version, "1", "la version del dominio cambio");
        assertEq(chainId, block.chainid, "el chainId del dominio cambio");
        assertEq(verifyingContract, address(distinciones), "el contrato del dominio cambio");

        return keccak256(
            abi.encode(
                keccak256(
                    "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"
                ),
                keccak256(bytes(name)),
                keccak256(bytes(version)),
                chainId,
                verifyingContract
            )
        );
    }

    function _digest(address to, uint16 pieza, uint64 deadline) internal view returns (bytes32) {
        bytes32 hashStruct = keccak256(abi.encode(TIPO_DISTINCION, to, pieza, deadline));
        return keccak256(abi.encodePacked(hex"1901", _separadorDominio(), hashStruct));
    }

    function _firmar(uint256 pk, address to, uint16 pieza, uint64 deadline)
        internal
        view
        returns (bytes memory)
    {
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(pk, _digest(to, pieza, deadline));
        return abi.encodePacked(r, s, v);
    }

    function _deadline() internal view returns (uint64) {
        return uint64(vm.getBlockTimestamp() + 10 minutes);
    }
}
